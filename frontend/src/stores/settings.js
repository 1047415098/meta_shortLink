import { ref } from "vue";
import { getSettings } from "../api/settings";
export const settings = ref({});
let pending;
export function loadSettings(force = false) {
  if (!force && settings.value.timezone) return Promise.resolve(settings.value);
  if (!pending)
    pending = getSettings()
      .then((value) => (settings.value = value))
      .finally(() => {
        pending = null;
      });
  return pending;
}
