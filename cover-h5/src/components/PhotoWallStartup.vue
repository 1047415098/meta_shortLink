<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { COVER_WALL_STAGES, nextCoverWallStage, nextLuckyUserNumber } from "../lib/coverWallFlow.js";

// 封面墙自动导入 photo 目录的全部图片；首帧从本地图片中随机选一张。
const photoFiles = import.meta.glob("../assets/photo/*.{jpg,jpeg,png,webp}", { eager:true, query:"?url", import:"default" });
const photoUrls = Object.values(photoFiles);
const initialPhoto = photoUrls[Math.floor(Math.random() * photoUrls.length)] || "";

const { t } = useI18n({ useScope:"global" });
const covers = ref([]);
// 入口先显示随机本地图片，photo 图片加载后再进入错峰网格与横向滚动墙。
const splashVisible = ref(true);
const splashLeaving = ref(false);
const splashTileCount = ref(24);
const splashTiles = ref([]);
// 封面墙从可操作的引导页直接开始，不读取倒计时主题的尾段时长。
const stage = ref(COVER_WALL_STAGES.INTRO);
const questionIndex = ref(0);
const selectedAnswer = ref(null);
// 已选答案留在聊天记录中，让用户自然看见问答进度，而不是显示数字步骤条。
const selectedAnswers = ref([]);
const chatViewport = ref(null);
const conversionButton = ref(null);
const paymentDialog = ref(null);
const paymentAddressText = ref(null);
const paymentOpen = ref(false);
const copyStatus = ref("");
const luckyNumber = ref(280);
// 运营提供的静态收款地址仅用于展示和复制，不表示站点具备到账核验或匹配服务。
const paymentAddress = "TSw6H1atFpEP2zWBR8NT3b6PGxpcAuQaTz";
// 六项占位奖品只参与界面轮亮；封面项目不会创建真实奖品，也没有小说内容绑定。
const prizeKeys = ["coverWallPrizeOne", "coverWallPrizeTwo", "coverWallPrizeThree", "coverWallPrizeFour", "coverWallPrizeFive", "coverWallPrizeSix"];
const revealActiveIndex = ref(-1);
const revealSettled = ref(false);
let questionTimer;
let revealTimer;
let splashTimer;
let splashExitTimer;
let mounted = true;
let previousBodyOverflow = "";

// 固定图片槽位的宽度；慢网时仍保留随机首帧，后续图片到达后逐张填入。
const fallbackCover = computed(() => initialPhoto || covers.value.find(Boolean) || "");
// 每行按总图片数分配槽位，保证 photo 中的每一张至少出现在滚动墙里。
const coversPerRow = computed(() => Math.max(10, Math.ceil(covers.value.length / 4)));
const wallRows = computed(() => covers.value.length
  ? Array.from({ length:4 }, (_, row) => Array.from({ length:coversPerRow.value }, (_, column) => covers.value[(row * coversPerRow.value + column) % covers.value.length]))
  : []);
// 占位文案有四条二选一提示；答案只推动当前独立封面流程。
const questions = computed(() => ([
  { prompt:t("coverWallQuestionOne"), options:[t("coverWallQuestionOneOptionOne"), t("coverWallQuestionOneOptionTwo")] },
  { prompt:t("coverWallQuestionTwo"), options:[t("coverWallQuestionTwoOptionOne"), t("coverWallQuestionTwoOptionTwo")] },
  { prompt:t("coverWallQuestionThree"), options:[t("coverWallQuestionThreeOptionOne"), t("coverWallQuestionThreeOptionTwo")] },
  { prompt:t("coverWallQuestionFour"), options:[t("coverWallQuestionFourOptionOne"), t("coverWallQuestionFourOptionTwo")] },
]));
const currentQuestion = computed(() => questions.value[questionIndex.value]);
function showNextStage() {
  const next = nextCoverWallStage(stage.value);
  if (next) stage.value = next;
  // 抽奖结束先打开收款信息；关闭后回到完整聊天记录，不跳转小说。
  if (next === COVER_WALL_STAGES.CONVERSION) {
    paymentOpen.value = true;
    void nextTick(() => paymentDialog.value?.showModal());
  }
}

function closeSplash() {
  if (!splashVisible.value || splashLeaving.value) return;
  splashLeaving.value = true;
  // 允许用户点按跳过首屏；淡出后才移除网格，避免引导页突然闪现。
  const reducedMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches;
  splashExitTimer = window.setTimeout(() => { splashVisible.value = false; }, reducedMotion ? 20 : 680);
}

function startQuestions() {
  // Answers only shape the visual path and never change campaign attribution.
  showNextStage();
}

function startRevealAnimation() {
  // 六项依次轮亮并随机停在其中一项；20 段减速共 6250ms，结果停留 1750ms。
  const spinCount = 21;
  const landingSlot = Math.floor(Math.random() * prizeKeys.length);
  revealSettled.value = false;
  if (window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches) {
    revealActiveIndex.value = landingSlot;
    revealSettled.value = true;
    // 减少动态效果时仍停留八秒，让窄屏用户有时间滚动查看完整占位文案。
    revealTimer = window.setTimeout(showNextStage, 8000);
    return;
  }
  // 从对应位置出发，轮完固定步数时恰好停在随机奖项，不在末帧突然跳格。
  revealActiveIndex.value = (landingSlot - spinCount % prizeKeys.length + prizeKeys.length) % prizeKeys.length;
  let spins = 0;
  const spin = () => {
    revealActiveIndex.value = (revealActiveIndex.value + 1) % prizeKeys.length;
    spins += 1;
    if (spins < spinCount) {
      revealTimer = window.setTimeout(spin, 50 + spins * 25);
      return;
    }
    revealSettled.value = true;
    revealTimer = window.setTimeout(showNextStage, 1750);
  };
  spin();
}

function chooseAnswer(index) {
  if (selectedAnswer.value !== null) return;
  selectedAnswer.value = index;
  questionTimer = window.setTimeout(() => {
    selectedAnswers.value.push(index);
    selectedAnswer.value = null;
    if (questionIndex.value + 1 < questions.value.length) {
      questionIndex.value += 1;
      // 新问题出现后滚到聊天底部，窄屏仍能看到当前选项。
      void nextTick(() => {
        if (chatViewport.value) chatViewport.value.scrollTop = chatViewport.value.scrollHeight;
      });
      return;
    }
    showNextStage();
    startRevealAnimation();
  }, 300);
}

function closePaymentDialog() {
  paymentOpen.value = false;
  // 对话较长时先滚到最后一条，确保福利按钮不会被手机视口裁掉。
  void nextTick(() => {
    if (chatViewport.value) chatViewport.value.scrollTop = chatViewport.value.scrollHeight;
    conversionButton.value?.focus({ preventScroll:true });
  });
}

function reopenPaymentDialog() {
  // 聊天页的福利按钮只重新展示现有弹窗，不触发小说跳转或支付确认。
  paymentOpen.value = true;
  void nextTick(() => paymentDialog.value?.showModal());
}

async function copyPaymentAddress() {
  // 安全环境使用 Clipboard API；普通 HTTP 预览页降级为选中文字复制。
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(paymentAddress);
    } else {
      const selection = window.getSelection();
      if (!paymentAddressText.value || !selection) throw new Error("copy unavailable");
      const range = document.createRange();
      range.selectNodeContents(paymentAddressText.value);
      selection.removeAllRanges();
      selection.addRange(range);
      if (!document.execCommand("copy")) throw new Error("copy failed");
      selection.removeAllRanges();
    }
    // 只保存文案键，弹窗和复制反馈始终跟随当前语言显示。
    copyStatus.value = "coverWallPaymentCopySuccess";
  } catch {
    copyStatus.value = "coverWallPaymentCopyFailure";
  }
}

// 单张图片下载完成后再显示；解码排队不阻塞其它图片进入墙面。
function preloadCover(url) {
  return new Promise((resolve) => {
    const image = new Image();
    let finished = false;
    let timer;
    const finish = (loaded) => {
      if (finished) return;
      finished = true;
      window.clearTimeout(timer);
      image.onload = image.onerror = null;
      resolve(loaded ? url : "");
    };
    image.onload = () => {
      // 计时只针对解码；慢网下载本身不应因固定秒数被误判成失败。
      timer = window.setTimeout(() => finish(image.naturalWidth > 0), 6000);
      if (image.decode) void image.decode().then(() => finish(true), () => finish(image.naturalWidth > 0));
      else finish(true);
    };
    image.onerror = () => finish(false);
    image.src = url;
  });
}

function loadCovers() {
  // 先打乱素材顺序；首屏图片一旦解码就逐张入场，不等待整批下载完成。
  const shuffled = [...photoUrls];
  for (let index = shuffled.length - 1; index > 0; index--) {
    const random = Math.floor(Math.random() * (index + 1));
    [shuffled[index], shuffled[random]] = [shuffled[random], shuffled[index]];
  }
  covers.value = Array(shuffled.length).fill("");
  shuffled.forEach(async (url, index) => {
    const ready = await preloadCover(url);
    if (!mounted || !ready) return;
    covers.value[index] = ready;
    if (index < splashTileCount.value) splashTiles.value[index] = ready;
  });
}

onMounted(() => {
  // 封面墙占满视口时锁住底层页面滚动；聊天记录仍在自身区域内正常滚动。
  previousBodyOverflow = document.body.style.overflow;
  document.body.style.overflow = "hidden";
  // sessionStorage 跨刷新保存上一次序号；禁用存储时仍可正常展示随机数字。
  try {
    const previous = window.sessionStorage.getItem("cover-wall-lucky-number");
    luckyNumber.value = nextLuckyUserNumber(previous);
    window.sessionStorage.setItem("cover-wall-lucky-number", String(luckyNumber.value));
  } catch {
    luckyNumber.value = nextLuckyUserNumber(null);
  }
  splashTileCount.value = window.innerWidth < 620 ? 24 : 30;
  splashTiles.value = Array(splashTileCount.value).fill("");
  splashTimer = window.setTimeout(closeSplash, window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ? 120 : 4600);
  void loadCovers();
});
onBeforeUnmount(() => {
  mounted = false;
  document.body.style.overflow = previousBodyOverflow;
  window.clearTimeout(splashTimer);
  window.clearTimeout(splashExitTimer);
  window.clearTimeout(questionTimer);
  window.clearTimeout(revealTimer);
});
</script>

<template>
  <section class="novel-photo-wall" :class="`is-${stage}`" :aria-label="t('openingStory')">
    <!-- 首帧和四行滚动墙均来自本地 photo；复制每行实现无缝循环。 -->
    <img v-if="fallbackCover" class="novel-photo-wall__fallback-cover" :src="fallbackCover" alt="" aria-hidden="true" />
    <span v-if="wallRows.length" class="novel-photo-wall__rows" aria-hidden="true">
      <span v-for="(row, rowIndex) in wallRows" :key="rowIndex" class="novel-photo-wall__row">
        <span class="novel-photo-wall__track">
          <span v-for="copy in 2" :key="copy" class="novel-photo-wall__group">
            <span v-for="(cover, index) in row" :key="`${copy}-${index}`" class="novel-photo-wall__cover">
              <img v-if="cover" :src="cover" alt="" decoding="async" />
            </span>
          </span>
        </span>
      </span>
    </span>
    <span class="novel-photo-wall__shade" aria-hidden="true" />

    <!-- 网格只负责首屏错峰入场；点按可以跳过，随后露出持续交错滚动的封面墙。 -->
    <button v-if="splashVisible" class="novel-photo-wall__splash" :class="{ 'is-leaving':splashLeaving }" type="button" :aria-label="t('coverWallSkipAnimation')" @click="closeSplash">
      <img v-if="fallbackCover" class="novel-photo-wall__splash-backdrop" :src="fallbackCover" alt="" aria-hidden="true" />
      <span class="novel-photo-wall__splash-grid" aria-hidden="true">
        <span v-for="(cover, index) in splashTiles" :key="index" class="novel-photo-wall__splash-tile" :class="{ 'is-ready':cover }" :style="{ '--photo-wall-delay': `${index * 42}ms` }">
          <img v-if="cover" :src="cover" alt="" decoding="async" />
        </span>
      </span>
    </button>

    <!-- 封面墙首屏由用户主动进入问答，与倒计时加载流程互不关联。 -->
    <article v-if="stage === COVER_WALL_STAGES.INTRO" class="novel-photo-wall__panel novel-photo-wall__intro" :inert="splashVisible ? '' : undefined" :aria-hidden="splashVisible ? 'true' : undefined">
      <!-- 长译文在文案区滚动，底部确认按钮始终留在窄屏可视区域。 -->
      <div class="novel-photo-wall__intro-copy">
        <p class="novel-photo-wall__eyebrow">{{ t('coverWallIntroEyebrow') }}</p>
        <h1>{{ t('coverWallIntroTitle') }}</h1>
        <p>{{ t('coverWallIntroDescription') }}</p>
        <!-- 三段欢迎语与竞品截图逐段对应，运营以后可单独改任意一段译文。 -->
        <p>{{ t('coverWallIntroDescriptionTwo') }}</p>
        <p>{{ t('coverWallIntroDescriptionThree') }}</p>
      </div>
      <button class="novel-photo-wall__button" type="button" @click="startQuestions">{{ t('coverWallContinue') }} <i class="fa-solid fa-arrow-right" /></button>
    </article>

    <!-- 抽奖后的收尾页复用问答聊天记录，不再出现小说封面或阅读按钮。 -->
    <article v-else-if="stage === COVER_WALL_STAGES.QUESTIONS || stage === COVER_WALL_STAGES.CONVERSION" class="novel-photo-wall__chat" :inert="paymentOpen ? '' : undefined" :aria-hidden="paymentOpen ? 'true' : undefined">
      <h1 class="novel-photo-wall__chat-heading">{{ t('coverWallQuestionHeading') }}</h1>
      <div ref="chatViewport" class="novel-photo-wall__chat-viewport">
        <div class="novel-photo-wall__chat-thread">
          <!-- 之前的问题与回答保留为左右消息气泡，当前问题始终位于聊天末尾。 -->
          <div v-for="(answerIndex, index) in selectedAnswers" :key="index" class="novel-photo-wall__chat-turn">
            <div class="novel-photo-wall__chat-row">
              <span class="novel-photo-wall__chat-avatar" aria-hidden="true"><i class="fa-solid fa-heart" /></span>
              <p class="novel-photo-wall__chat-bubble novel-photo-wall__chat-bubble--prompt">{{ questions[index].prompt }}</p>
            </div>
            <div class="novel-photo-wall__chat-row novel-photo-wall__chat-row--answer">
              <p class="novel-photo-wall__chat-bubble novel-photo-wall__chat-bubble--answer">{{ questions[index].options[answerIndex] }}</p>
            </div>
          </div>
          <div v-if="stage === COVER_WALL_STAGES.QUESTIONS" class="novel-photo-wall__chat-turn" aria-live="polite">
            <div class="novel-photo-wall__chat-row">
              <span class="novel-photo-wall__chat-avatar" aria-hidden="true"><i class="fa-solid fa-heart" /></span>
              <p class="novel-photo-wall__chat-bubble novel-photo-wall__chat-bubble--prompt">{{ currentQuestion.prompt }}</p>
            </div>
            <div class="novel-photo-wall__chat-choices">
              <button v-for="(option, index) in currentQuestion.options" :key="option" class="novel-photo-wall__answer" :class="{ 'is-selected':selectedAnswer === index }" type="button" :disabled="selectedAnswer !== null" @click="chooseAnswer(index)">
                {{ option }}
              </button>
            </div>
          </div>
          <!-- 最后一题结束后仅提供重新查看福利的入口，完整问答仍可向上滚动回看。 -->
          <button v-else ref="conversionButton" class="novel-photo-wall__benefit-button" type="button" @click="reopenPaymentDialog">{{ t('coverWallViewBenefits') }}</button>
        </div>
      </div>
    </article>

    <article v-else-if="stage === COVER_WALL_STAGES.REVEAL" class="novel-photo-wall__panel novel-photo-wall__reveal">
      <p class="novel-photo-wall__eyebrow">{{ t('coverWallRevealEyebrow') }}</p>
      <h1>{{ t('coverWallRevealTitle', { number:luckyNumber }) }}</h1>
      <p>{{ t('coverWallRevealDescription') }}</p>
      <!-- 六项奖品是唯一轮亮区域；只显示抽中的占位文案，不与兑奖或支付流程绑定。 -->
      <ul class="novel-photo-wall__prizes">
        <li v-for="(key, index) in prizeKeys" :key="key" :class="{ 'is-active':revealActiveIndex === index, 'is-final':revealSettled && revealActiveIndex === index }" :aria-current="revealSettled && revealActiveIndex === index ? 'true' : undefined">{{ t(key) }}</li>
      </ul>
      <p class="novel-photo-wall__drawing" role="status" aria-live="polite">{{ revealSettled ? t(prizeKeys[revealActiveIndex]) : t('coverWallDrawing') }}</p>
    </article>

    <!-- 静态收款信息与反馈使用同一份语言包；只展示和复制，不自动确认付款。 -->
    <dialog v-if="stage === COVER_WALL_STAGES.CONVERSION && paymentOpen" ref="paymentDialog" class="novel-photo-wall__payment-dialog" aria-labelledby="cover-wall-payment-title" aria-describedby="cover-wall-payment-description" @close="closePaymentDialog">
      <div class="novel-photo-wall__payment-card">
        <button class="novel-photo-wall__payment-close" type="button" :aria-label="t('coverWallPaymentClose')" @click="paymentDialog?.close()"><i class="fa-solid fa-xmark" aria-hidden="true" /></button>
        <p class="novel-photo-wall__payment-eyebrow">{{ t('coverWallPaymentEyebrow') }}</p>
        <h2 id="cover-wall-payment-title">{{ t('coverWallPaymentTitle') }}</h2>
        <p id="cover-wall-payment-description" class="novel-photo-wall__payment-description">{{ t('coverWallPaymentDescription') }}</p>
        <p class="novel-photo-wall__payment-label">{{ t('coverWallPaymentAddressLabel') }}</p>
        <div class="novel-photo-wall__payment-address-row">
          <!-- 可选中文字允许完整地址在窄屏换行，避免单行字段截断收款信息。 -->
          <p id="cover-wall-payment-address" ref="paymentAddressText" class="novel-photo-wall__payment-address-value">{{ paymentAddress }}</p>
          <button class="novel-photo-wall__payment-copy" type="button" @click="copyPaymentAddress">{{ t('coverWallPaymentCopy') }}</button>
        </div>
        <p class="novel-photo-wall__payment-notice">{{ t('coverWallPaymentNotice') }}</p>
        <p class="novel-photo-wall__payment-copy-status" role="status" aria-live="polite">{{ copyStatus ? t(copyStatus) : '' }}</p>
      </div>
    </dialog>
  </section>
</template>
