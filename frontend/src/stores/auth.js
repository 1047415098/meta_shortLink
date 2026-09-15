import { ref } from "vue";
import { getSession, signIn, signOut } from "../api/auth";
export const user = ref(null);
let pending;
export async function ensureSession() {
  if (user.value) return true;
  if (!pending)
    pending = getSession()
      .then((v) => ((user.value = v), true))
      .catch((e) => {
        if (e.status === 401) return false;
        throw e;
      })
      .finally(() => (pending = null));
  return pending;
}
export async function login(credentials) {
  await signIn(credentials);
  user.value = await getSession();
}
export async function logout() {
  await signOut();
  user.value = null;
}
