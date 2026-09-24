<template>
  <section>
    <PageHeader
      title="语音小说管理"
      context="语音文学内容"
      description="维护独立语音小说站展示的全局内容。"
    >
      <el-button :icon="Refresh" :loading="loading" @click="load"
        >刷新</el-button
      >
      <el-button
        type="primary"
        :icon="Plus"
        @click="router.push({ name: 'audio-novel-create' })"
        >新增语音小说</el-button
      >
    </PageHeader>
    <section class="panel" v-loading="loading">
      <div class="audio-novel-toolbar">
        <el-input
          v-model="filters.q"
          clearable
          placeholder="搜索标题、slug 或分类"
          @keyup.enter="search"
        />
        <el-select v-model="filters.status" @change="search"
          ><el-option label="全部状态" value="" /><el-option
            label="已启用"
            value="enabled" /><el-option label="已停用" value="disabled"
        /></el-select>
        <el-button @click="search">查询</el-button>
      </div>
      <el-table :data="result.items" empty-text="暂无语音小说">
        <el-table-column label="封面" width="92"
          ><template #default="{ row }"
            ><el-image class="cover" :src="row.cover_path" fit="cover"
              ><template #error
                ><div class="cover-empty">无封面</div></template
              ></el-image
            ></template
          ></el-table-column
        >
        <el-table-column label="语音小说" min-width="240"
          ><template #default="{ row }"
            ><b>{{ row.title }}</b>
            <div class="muted">
              {{ row.slug }} · {{ row.category }}
            </div></template
          ></el-table-column
        >
        <el-table-column prop="published_at" label="发布日期" width="125" />
        <el-table-column label="Podcast" width="150">
          <template #default="{ row }">
            <el-tag v-if="row.audio_path" type="success">
              已上传 · {{ row.audio_duration }}
            </el-tag>
            <span v-else class="muted">无音频</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="95"
          ><template #default="{ row }"
            ><el-tag :type="row.enabled ? 'success' : 'info'">{{
              row.enabled ? "启用" : "停用"
            }}</el-tag></template
          ></el-table-column
        >
        <el-table-column label="推荐" width="85"
          ><template #default="{ row }"
            ><el-tag v-if="row.featured" type="warning">首页</el-tag
            ><span v-else>—</span></template
          ></el-table-column
        >
        <el-table-column label="操作" width="350" fixed="right"
          ><template #default="{ row }">
            <el-button
              link
              type="primary"
              @click="
                router.push({
                  name: 'audio-novel-links',
                  params: { id: row.id },
                })
              "
              >投放链接</el-button
            >
            <el-button
              link
              type="primary"
              @click="
                router.push({
                  name: 'audio-novel-edit',
                  params: { id: row.id },
                })
              "
              >编辑</el-button
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
  deleteAudioNovel,
  listAudioNovels,
  setAudioNovelEnabled,
  setAudioNovelFeatured,
} from "../api/audioNovels.js";
const router = useRouter(),
  loading = ref(false),
  filters = reactive({ q: "", status: "", page: 1, page_size: 20 }),
  result = reactive({ items: [], page: 1, page_size: 20, total: 0 });
async function load() {
  loading.value = true;
  try {
    Object.assign(result, await listAudioNovels(filters));
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
    await setAudioNovelEnabled(row.id, !row.enabled);
    ElMessage.success("状态已更新");
    load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function toggleFeatured(row) {
  try {
    await setAudioNovelFeatured(row.id, !row.featured);
    ElMessage.success("推荐状态已更新");
    load();
  } catch (error) {
    ElMessage.error(error.message);
  }
}
async function remove(row) {
  try {
    await ElMessageBox.confirm(
      `确认删除《${row.title}》？删除后公开页面将不可访问。`,
      "删除语音小说",
      { type: "warning" },
    );
    await deleteAudioNovel(row.id);
    if (result.items.length === 1 && filters.page > 1) filters.page--;
    ElMessage.success("语音小说已删除");
    load();
  } catch (error) {
    if (error !== "cancel" && error !== "close")
      ElMessage.error(error.message || "删除失败");
  }
}
onMounted(load);
</script>

<style scoped>
.audio-novel-toolbar {
  display: grid;
  grid-template-columns: minmax(240px, 1fr) 160px auto;
  gap: 12px;
  margin-bottom: 20px;
}
.cover {
  width: 60px;
  height: 72px;
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
  .audio-novel-toolbar {
    grid-template-columns: 1fr;
  }
  .panel {
    overflow-x: auto;
  }
}
</style>
