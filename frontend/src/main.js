import { ElUpload } from "element-plus/es/components/upload/index";
import {
  ElRadio,
  ElRadioGroup,
  ElRadioButton,
} from "element-plus/es/components/radio/index";
import { ElProgress } from "element-plus/es/components/progress/index";
import { ElStatistic } from "element-plus/es/components/statistic/index";
import { ElCard } from "element-plus/es/components/card/index";
import { ElDatePicker } from "element-plus/es/components/date-picker/index";
import { ElDivider } from "element-plus/es/components/divider/index";
import {
  ElDropdown,
  ElDropdownMenu,
  ElDropdownItem,
} from "element-plus/es/components/dropdown/index";
import { ElAvatar } from "element-plus/es/components/avatar/index";
import {
  ElBreadcrumb,
  ElBreadcrumbItem,
} from "element-plus/es/components/breadcrumb/index";
import { ElIcon } from "element-plus/es/components/icon/index";
import { ElMenu, ElMenuItem } from "element-plus/es/components/menu/index";
import { ElHeader } from "element-plus/es/components/container/index";
import { ElAside } from "element-plus/es/components/container/index";
import { ElContainer } from "element-plus/es/components/container/index";
import { createApp } from "vue";
import { ElAlert } from "element-plus/es/components/alert/index";
import { ElButton } from "element-plus/es/components/button/index";
import { ElInput } from "element-plus/es/components/input/index";
// 停留阈值使用数字输入组件；本项目显式注册组件，避免生产环境渲染为空。
import { ElInputNumber } from "element-plus/es/components/input-number/index";
import { ElSelect } from "element-plus/es/components/select/index";
import { ElOption } from "element-plus/es/components/select/index";
import { ElEmpty } from "element-plus/es/components/empty/index";
// 小说与语音小说封面依赖图片组件；显式注册后才能生成实际的 img 元素。
import { ElImage } from "element-plus/es/components/image/index";
import { ElTable } from "element-plus/es/components/table/index";
import { ElTableColumn } from "element-plus/es/components/table/index";
import { ElTag } from "element-plus/es/components/tag/index";
import { ElTooltip } from "element-plus/es/components/tooltip/index";
import { ElPagination } from "element-plus/es/components/pagination/index";
import { ElDescriptions } from "element-plus/es/components/descriptions/index";
import { ElDescriptionsItem } from "element-plus/es/components/descriptions/index";
import { ElDialog } from "element-plus/es/components/dialog/index";
import { ElForm } from "element-plus/es/components/form/index";
import { ElFormItem } from "element-plus/es/components/form/index";
import { ElSwitch } from "element-plus/es/components/switch/index";
import { ElCheckbox, ElCheckboxGroup } from "element-plus/es/components/checkbox/index";
import { ElLoading } from "element-plus/es/components/loading/index";
import { ElConfigProvider } from "element-plus/es/components/config-provider/index";
import "element-plus/dist/index.css";
import App from "./App.vue";
import "./styles/base.css";
import router from "./router";
const app = createApp(App);
for (const component of [
  ElContainer,
  ElAside,
  ElHeader,
  ElMenu,
  ElMenuItem,
  ElIcon,
  ElBreadcrumb,
  ElBreadcrumbItem,
  ElAvatar,
  ElDropdown,
  ElDropdownMenu,
  ElDropdownItem,
  ElDivider,
  ElDatePicker,
  ElCard,
  ElStatistic,
  ElProgress,
  ElRadio,
  ElRadioGroup,
  ElRadioButton,
  ElUpload,
  ElAlert,
  ElButton,
  ElInput,
  ElInputNumber,
  ElSelect,
  ElOption,
  ElEmpty,
  ElImage,
  ElTable,
  ElTableColumn,
  ElTag,
  ElTooltip,
  ElPagination,
  ElDescriptions,
  ElDescriptionsItem,
  ElDialog,
  ElForm,
  ElFormItem,
  ElSwitch,
  ElCheckbox,
  ElCheckboxGroup,
  ElConfigProvider,
])
  app.use(component);
app.use(ElLoading).use(router).mount("#app");
