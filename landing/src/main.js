import { createApp } from "vue";
import App from "./App.vue";
import "./styles.css";

// Go has already resolved the public short-code route and injected its state,
// so the landing bundle does not need a client-side router.
createApp(App).mount("#app");
