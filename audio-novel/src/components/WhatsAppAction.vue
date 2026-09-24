<script setup>
import { inject, ref } from "vue";
import { submitAudioNovelContact } from "../lib/contact.js";
import { trackMetaConsult } from "../lib/meta.js";

const bootstrap = inject("bootstrap");
const busy = ref(false);
const message = ref("");

async function openWhatsApp() {
  if (busy.value) return;
  busy.value = true;
  message.value = "";
  try {
	trackMetaConsult(bootstrap.meta_manual_event_id);
    let target = bootstrap.link.target_url;
    if (bootstrap.ticket) {
      const result = await submitAudioNovelContact({ code: bootstrap.link.code, ticket: bootstrap.ticket });
      target = result.target_url;
    }
    window.location.assign(target);
  } catch (error) {
    message.value = error.message;
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <!-- New audio campaign links intentionally have no WhatsApp destination;
       legacy archive links keep rendering the action from their target URL. -->
  <div v-if="bootstrap.link?.target_url" class="contact-action">
    <button class="whatsapp-button" type="button" :disabled="busy" @click="openWhatsApp">
      {{ busy ? "Opening…" : "Continue on WhatsApp" }}
    </button>
    <p v-if="message" class="action-error" role="status">{{ message }}</p>
  </div>
</template>
