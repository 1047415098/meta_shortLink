import { ref, reactive, onMounted, onBeforeUnmount } from "vue";
import { listLinks } from "../api/links";
import { exportVisits } from "../api/analytics";
import { settings } from "../stores/settings";
import { DEFAULT_REPORT_TIMEZONE } from "../constants/reportTimezones";
import { ElMessage } from "element-plus/es/components/message/index";
export function useReport(fetcher) {
  const defaults = () => {
    const end = new Date().toLocaleDateString("en-CA", {
      // Use fixed UTC-8 until the settings request supplies the deployed choice.
      timeZone: settings.value.timezone || DEFAULT_REPORT_TIMEZONE,
    });
    // Reports open on the current calendar day; historical ranges are an explicit operator choice.
    return {
      start: end,
      end,
      tz: settings.value.timezone || DEFAULT_REPORT_TIMEZONE,
      link_id: "",
      ad_id: "",
      surface: "",
    };
  };
  // Report state stays in memory so date and ad filters never enter the URL.
  const filters = reactive(defaults()),
    data = ref(null),
    links = ref([]),
    busy = ref(false),
    error = ref(""),
    clickPage = ref(1);
  let generation = 0,
    alive = true;
  async function load(reset = true) {
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
  onMounted(async () => {
    try {
      links.value = await listLinks();
      await load(false);
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
