<template>
  <section>
    <PageHeader
      title="小说管理"
      context="免费小说内容"
      description="维护免费 H5 小说站的书籍和章节内容。"
    >
      <el-button :icon="Refresh" :loading="loading" @click="load"
        >刷新</el-button
      >
      <el-button
        type="primary"
        :icon="Plus"
        @click="router.push({ name: 'novel-create' })"
        >新增小说</el-button
      >
    </PageHeader>
    <section class="panel" v-loading="loading">
      <div class="toolbar">
        <el-input
          v-model="filters.q"
          clearable
          placeholder="搜索标题、作者或 slug"
          @keyup.enter="search"
        />
        <el-select v-model="filters.status" @change="search"
          ><el-option label="全部状态" value="" /><el-option
            label="已启用"
            value="enabled" /><el-option label="已停用" value="disabled"
        /></el-select>
        <el-button @click="search">查询</el-button>
      </div>
      <el-table :data="result.items" empty-text="暂无小说">
        <el-table-column label="封面" width="82"
          ><template #default="{ row }"
            ><el-image
              v-if="row.cover_path"
              class="cover"
              :src="row.cover_path"
              :preview-src-list="[row.cover_path]"
              preview-teleported
              fit="cover"
              ><template #error
                ><div class="cover-empty">加载失败</div></template
              ></el-image
            >
            <div v-else class="cover cover-empty">无封面</div></template
          ></el-table-column
        >
        <el-table-column label="小说" min-width="240"
          ><template #default="{ row }"
            ><b>{{ row.title }}</b>
            <div class="muted">{{ row.author }} · {{ row.slug }}</div></template
          ></el-table-column
        >
        <el-table-column prop="category" label="分类" width="120" />
        <el-table-column prop="chapter_count" label="章节" width="75" />
        <el-table-column prop="sort_order" label="排序" width="75" />
        <el-table-column label="状态" width="90"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "启用" : "停用"
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="推荐" width="80"
          ><template #default="{ row }"
            ><el-tag v-if="row.featured" type="warning">首页</el-tag
            ><span v-else>—</span></template
          ></el-table-column
        >
        <el-table-column label="操作" width="340" fixed="right"
          ><template #default="{ row }">
            <el-button
              link
              type="primary"
              @click="
                router.push({ name: 'novel-links', params: { id: row.id } })
              "
              >投放链接</el-button
            >
            <el-button
              link
              type="primary"
              @click="
                router.push({ name: 'novel-edit', params: { id: row.id } })
              "
              >编辑/章节</el-button
            >
            <el-button
              link
              :type="row.enabled ? 'danger' : 'success'"
              @click="toggleEnabled(row)"
              >{{ row.enabled ? "停用" : "启用" }}</el-button
            >
            <el-button
              link
              :disabled="!row.enabled"
              @click="toggleFeatured(row)"
              >{{ row.featured ? "取消推荐" : "设为推荐" }}</el-button
            >
            <el-button link type="danger" @click="remove(row)">删除</el-button>
          </template></el-table-column
        >
      </el-table>
      <el-pagination
        v-if="result.total"
        class="pager"
        background
        layout="prev, pager, next, total"
        :current-page="result.page"
        :page-size="result.page_size"
        :total="result.total"
        @current-change="changePage"
      />
    </section>
  </section>
</template>

<script setup>
import { onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { Plus, Refresh } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import PageHeader from "../components/PageHeader.vue";
import {
  deleteNovel,
  listNovels,
  setNovelEnabled,
  setNovelFeatured,
} from "../api/novels.js";
const router = useRouter(),
  loading = ref(false);
const filters = reactive({ q: "", status: "", page: 1, page_size: 20 });
const result = reactive({ items: [], page: 1, page_size: 20, total: 0 });
async function load() {
  loading.value = true;
  try {
    Object.assign(result, await listNovels(filters));
  } catch (error) {
    ElMessage.error(error.message);
  } finally {
    loading.value = false;
  }
}
function search() {
  filters.page = 1;
  load();
}
function changePage(page) {
  filters.page = page;
  load();
}
async function toggleEnabled(row) {
  try {
    await setNovelEnabled(row.id, !row.enabled);
    ElMessage.success("状态已更新");
    load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function toggleFeatured(row) {
  try {
    await setNovelFeatured(row.id, !row.featured);
    ElMessage.success("推荐状态已更新");
    load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function remove(row) {
  try {
    await ElMessageBox.confirm(
      `确认删除《${row.title}》及其全部章节？`,
      "删除小说",
      { type: "warning" },
    );
    await deleteNovel(row.id);
    if (result.items.length === 1 && filters.page > 1) filters.page--;
    ElMessage.success("小说已删除");
    load();
  } catch (error) {
    if (error !== "cancel" && error !== "close")
      ElMessage.error(error.message || "删除失败");
  }
}
onMounted(load);
</script>

<style scoped>
.toolbar {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) 160px auto;
  gap: 12px;
  margin-bottom: 20px;
}
.cover {
  width: 52px;
  height: 68px;
  border-radius: 5px;
  background: #f2f3f5;
}
.cover-empty {
  height: 100%;
  display: grid;
  place-items: center;
  color: #909399;
  font-size: 11px;
}
.pager {
  justify-content: flex-end;
  margin-top: 20px;
}
@media (max-width: 760px) {
  .toolbar {
    grid-template-columns: 1fr;
  }
  .panel {
    overflow-x: auto;
  }
}
</style>
