import { ref, reactive, onMounted, onBeforeUnmount, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { listLinks } from "../api/links";
import { exportVisits } from "../api/analytics";
import { settings } from "../stores/settings";
import { ElMessage } from "element-plus/es/components/message/index";
export function useReport(fetcher) {
  const route = useRoute(),
    router = useRouter();
  const defaults = () => {
    const end = new Date().toLocaleDateString("en-CA", {
      timeZone: settings.value.timezone || "Asia/Shanghai",
    });
    const d = new Date(end + "T12:00:00Z");
    d.setUTCDate(d.getUTCDate() - 6);
    return {
      start: d.toISOString().slice(0, 10),
      end,
      tz: settings.value.timezone || "Asia/Shanghai",
      link_id: "",
      ad_id: "",
    };
  };
  const fromQuery = () => {
    const v = defaults();
    for (const key of Object.keys(v))
      if (typeof route.query[key] === "string") v[key] = route.query[key];
    return v;
  };
  const filters = reactive(fromQuery()),
    data = ref(null),
    links = ref([]),
    busy = ref(false),
    error = ref(""),
    clickPage = ref(Number(route.query.page) || 1);
  let generation = 0,
    alive = true;
  async function load(reset = true, sync = true) {
    if (reset) clickPage.value = 1;
    const run = ++generation;
    error.value = "";
    if (!filters.start || !filters.end || filters.start > filters.end) {
      error.value = "请选择有效日期范围";
      busy.value = false;
      return;
    }
    busy.value = true;
    try {
      const snapshot = Object.fromEntries(
        Object.entries(filters).map(([key, value]) => [
          key,
          String(value ?? ""),
        ]),
      );
      Object.assign(filters, snapshot);
      if (sync)
        await router.replace({
          query: { ...snapshot, page: String(clickPage.value) },
        });
      const result = await fetcher({ ...snapshot, page: clickPage.value });
      if (alive && run === generation) data.value = result;
    } catch (e) {
      if (alive && run === generation) error.value = e.message;
    } finally {
      if (alive && run === generation) busy.value = false;
    }
  }
  function resetFilters() {
    Object.assign(filters, defaults());
    load();
  }
  async function exportCSV() {
    try {
      const blob = await exportVisits(filters);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "clicks-" + filters.start + "-" + filters.end + ".csv";
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      ElMessage.error(e.message);
    }
  }
  watch(
    () => route.query,
    () => {
      if (!alive) return;
      const next = fromQuery();
      const nextPage = Number(route.query.page) || 1;
      if (
        JSON.stringify(next) !== JSON.stringify({ ...filters }) ||
        nextPage !== clickPage.value
      ) {
        Object.assign(filters, next);
        clickPage.value = nextPage;
        load(false, false);
      }
    },
  );
  onMounted(async () => {
    try {
      links.value = await listLinks();
      await load(false, false);
    } catch (e) {
      error.value = e.message;
    }
  });
  onBeforeUnmount(() => {
    alive = false;
    generation++;
  });
  return {
    filters,
    data,
    links,
    busy,
    error,
    clickPage,
    load,
    resetFilters,
    exportCSV,
  };
}
