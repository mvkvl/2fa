import { api, Events } from "./bindings.js?v=8";

const $ = (id) => document.getElementById(id);
const NS = "http://www.w3.org/2000/svg";
const THEME_KEY = "2fa.theme";
const THEME_MODE_KEY = "2fa.theme-mode";
const THEMES = new Set(["mint", "midnight", "ocean", "sand"]);
const THEME_MODES = new Set(["auto", "light", "dark"]);
const THEME_MODE_ORDER = ["auto", "light", "dark"];
const LEGACY_THEMES = {
  system: ["mint", "auto"],
  dark: ["mint", "dark"],
  light: ["mint", "light"],
  midnight: ["midnight", "dark"],
  ocean: ["ocean", "dark"],
  sand: ["sand", "light"],
};
const systemTheme = window.matchMedia ? window.matchMedia("(prefers-color-scheme: light)") : null;

const state = {
  status: null,
  accounts: [],
  search: "",
  editing: "",
  importSelection: null,
  importPassphrase: "",
  toastTimer: null,
};

function withTimeout(promise, ms, message) {
  return Promise.race([
    promise,
    new Promise((_, reject) => setTimeout(() => reject(new Error(message)), ms)),
  ]);
}

function setScreen(name) {
  for (const screen of document.querySelectorAll(".screen")) {
    const active = screen.id === name;
    screen.classList.toggle("active", active);
    screen.classList.toggle("hidden", !active);
    screen.hidden = !active;
    screen.setAttribute("aria-hidden", String(!active));
  }
}

function toastHost() {
  const dialogs = document.querySelectorAll("dialog[open]");
  const dialog = dialogs.length ? dialogs[dialogs.length - 1] : null;
  const card = dialog ? dialog.querySelector(".modal-card") : null;
  return card || document.body;
}

function showToast(message, kind = "ok") {
  const toast = $("toast");
  if (!toast) return;
  const host = toastHost();
  if (toast.parentElement !== host) {
    if (host === document.body) {
      document.body.appendChild(toast);
    } else {
      host.insertBefore(toast, host.firstChild);
    }
  }
  clearTimeout(state.toastTimer);
  toast.textContent = message;
  toast.className = `toast show ${kind}`;
  state.toastTimer = setTimeout(() => {
    toast.className = "toast";
  }, 4200);
}

function errorText(err) {
  if (!err) return "Неизвестная ошибка";
  if (typeof err === "string") return err;
  return err.message || String(err);
}

function normalizeTheme(value) {
  return THEMES.has(value) ? value : "mint";
}

function normalizeThemeMode(value) {
  return THEME_MODES.has(value) ? value : "auto";
}

function resolveThemeMode(mode) {
  if (mode === "auto") {
    return systemTheme?.matches ? "light" : "dark";
  }
  return mode;
}

function themeModeLabel(mode) {
  if (mode === "light") return "светлая";
  if (mode === "dark") return "тёмная";
  return "как в системе";
}

function updateThemeModeControl(mode) {
  const button = $("themeModeBtn");
  if (button) {
    button.dataset.themeMode = mode;
    button.title = `Режим: ${themeModeLabel(mode)}`;
    button.setAttribute("aria-label", `Режим темы: ${themeModeLabel(mode)}`);
  }
  const hint = $("themeModeHint");
  if (hint) hint.textContent = `Режим: ${themeModeLabel(mode)}. Нажмите на кнопку, чтобы переключить.`;
}

function loadThemeSettings() {
  let theme = "mint";
  let mode = "auto";
  let stored = true;
  let migrated = false;
  try {
    const savedTheme = localStorage.getItem(THEME_KEY);
    const savedMode = localStorage.getItem(THEME_MODE_KEY);
    if (THEMES.has(savedTheme)) {
      theme = savedTheme;
    } else if (LEGACY_THEMES[savedTheme]) {
      theme = LEGACY_THEMES[savedTheme][0];
      mode = LEGACY_THEMES[savedTheme][1];
      migrated = true;
    } else {
      stored = false;
    }
    if (THEME_MODES.has(savedMode)) mode = savedMode;
    else stored = false;
  } catch {
    stored = false;
  }
  return { theme, mode, stored: stored && !migrated };
}

function applyTheme(theme, mode, persist = true) {
  const root = document.documentElement;
  const nextTheme = normalizeTheme(theme);
  const nextMode = normalizeThemeMode(mode);
  root.dataset.theme = nextTheme;
  root.dataset.themeMode = nextMode;
  root.dataset.mode = resolveThemeMode(nextMode);
  if (persist) {
    try {
      localStorage.setItem(THEME_KEY, nextTheme);
      localStorage.setItem(THEME_MODE_KEY, nextMode);
    } catch {
      // Theme persistence is optional.
    }
  }
  const select = $("themeSelect");
  if (select) select.value = nextTheme;
  updateThemeModeControl(nextMode);
}

function currentTheme() {
  return normalizeTheme(document.documentElement.dataset.theme);
}

function currentThemeMode() {
  return normalizeThemeMode(document.documentElement.dataset.themeMode);
}

function syncSystemTheme() {
  if (currentThemeMode() === "auto") {
    document.documentElement.dataset.mode = resolveThemeMode("auto");
  }
}

function cycleThemeMode() {
  const index = THEME_MODE_ORDER.indexOf(currentThemeMode());
  const next = THEME_MODE_ORDER[(index + 1) % THEME_MODE_ORDER.length];
  applyTheme(currentTheme(), next);
}

if (systemTheme) {
  if (systemTheme.addEventListener) systemTheme.addEventListener("change", syncSystemTheme);
  else if (systemTheme.addListener) systemTheme.addListener(syncSystemTheme);
}

async function closeWindow() {
  try {
    await api.closeWindow();
  } catch (err) {
    showToast(`Не удалось закрыть окно: ${errorText(err)}`, "error");
  }
}

function openDialog(dialog) {
  if (!dialog) return;
  if (!dialog.open) dialog.showModal();
}

function closeDialog(dialog) {
  if (dialog?.open) dialog.close();
}

function setBusy(button, busy, label = "…") {
  if (!button) return;
  if (busy) {
    button.dataset.label = button.textContent;
    button.disabled = true;
    button.textContent = label;
  } else {
    button.disabled = false;
    if (button.dataset.label) button.textContent = button.dataset.label;
  }
}

function plural(n, one, few, many) {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return few;
  return many;
}

function buildRing() {
  const group = $("ringSegments");
  if (!group) return;
  group.replaceChildren();
  for (let i = 0; i < 30; i++) {
    const line = document.createElementNS(NS, "line");
    line.setAttribute("x1", "24");
    line.setAttribute("y1", "6");
    line.setAttribute("x2", "24");
    line.setAttribute("y2", "10");
    line.setAttribute("transform", `rotate(${i * 12} 24 24)`);
    line.setAttribute("class", "ring-segment");
    group.appendChild(line);
  }
  updateRing();
}

function updateRing() {
  const seconds = $("ringSeconds");
  if (!seconds) return;
  const remaining = 30 - (Math.floor(Date.now() / 1000) % 30);
  seconds.textContent = String(remaining);
  const ring = seconds.closest(".ring");
  if (ring) {
    ring.dataset.phase = remaining <= 5 ? "crit" : remaining <= 10 ? "warn" : "ok";
  }
  const segments = $("ringSegments")?.children || [];
  for (let i = 0; i < segments.length; i++) {
    segments[i].classList.toggle("on", i < remaining);
  }
}

async function loadProviders() {
  try {
    const providers = await api.providers();
    const select = $("createProvider");
    if (!select) return;
    const previous = select.value;
    select.replaceChildren();
    for (const provider of providers) {
      const option = document.createElement("option");
      option.value = provider.name;
      option.textContent = provider.label;
      option.disabled = !provider.available;
      select.appendChild(option);
    }
    const preferred = providers.find((p) => p.recommended && p.available) || providers.find((p) => p.available);
    select.value = previous || preferred?.name || "passphrase";
    const keyring = providers.find((p) => p.name === "keyring");
    const note = $("keyringNote");
    if (note) {
      note.textContent = keyring?.available
        ? "Ключ будет храниться в системном keyring. Резервная копия — только через экспорт .2fa."
        : "Keyring недоступен. Будет использован пароль vault.";
    }
    updateCreateFields();
  } catch (err) {
    showToast(`Не удалось получить провайдеров: ${errorText(err)}`, "error");
  }
}

function updateCreateFields() {
  const provider = $("createProvider")?.value;
  const passphraseFields = $("createPassphraseFields");
  if (passphraseFields) passphraseFields.hidden = provider !== "passphrase";
  if (passphraseFields) passphraseFields.classList.toggle("hidden", provider !== "passphrase");
}

function updateUnlockScreen() {
  const status = state.status || {};
  const hint = $("unlockHint");
  const wrap = $("unlockPassphraseWrap");
  if (!hint) return;
  if (status.error) {
    hint.textContent = status.error;
    hint.className = "form-hint error";
  } else if (status.provider === "passphrase") {
    hint.textContent = "Введите пароль vault, чтобы разблокировать аккаунты.";
    hint.className = "form-hint";
  } else if (status.provider === "keyring") {
    hint.textContent = "Хранилище защищено системным keyring. Нажмите «Разблокировать».";
    hint.className = "form-hint";
  } else if (status.plaintext) {
    hint.textContent = "Хранилище не зашифровано. Нажмите «Открыть», чтобы продолжить.";
    hint.className = "form-hint warn";
  } else {
    hint.textContent = "Введите пароль vault.";
    hint.className = "form-hint";
  }
  if (wrap) wrap.hidden = status.provider !== "passphrase";
  if (wrap) wrap.classList.toggle("hidden", status.provider !== "passphrase");
}

async function refreshStatus(autoUnlockKeyring = false) {
  try {
    state.status = await withTimeout(api.status(), 5000, "Wails не ответил при чтении состояния хранилища");
  } catch (err) {
    showToast(`Не удалось прочитать состояние хранилища: ${errorText(err)}`, "error");
    setScreen("unlock");
    return;
  }

  if (autoUnlockKeyring && state.status.exists && state.status.locked && state.status.provider === "keyring" && state.status.keyringAvailable) {
    try {
      state.status = await withTimeout(api.unlock(""), 8000, "Wails не ответил при разблокировке keyring");
    } catch {
      // The keyring may contain no key for this vault; show the unlock screen.
    }
  }

  const status = state.status;
  $("vaultInfo").textContent = status.path || "";

  if (!status.exists) {
    setScreen("onboarding");
    await loadProviders();
    return;
  }
  if (status.locked) {
    updateUnlockScreen();
    setScreen("unlock");
    return;
  }
  setScreen("main");
  await loadAccounts();
}

async function loadAccounts() {
  try {
    state.accounts = await api.listAccounts();
    renderAccounts();
  } catch (err) {
    state.accounts = [];
    renderAccounts();
    showToast(`Не удалось загрузить аккаунты: ${errorText(err)}`, "error");
  }
}

function renderAccounts() {
  const list = $("accountList");
  const empty = $("emptyState");
  if (!list) return;
  list.replaceChildren();
  const query = state.search.trim().toLowerCase();
  const accounts = state.accounts.filter((account) => (account.name + " " + (account.description || "")).toLowerCase().includes(query));
  if (empty) {
    empty.hidden = accounts.length !== 0;
    empty.classList.toggle("hidden", accounts.length !== 0);
  }
  for (const account of accounts) {
    list.appendChild(renderAccount(account));
  }
}

async function copyAccount(account) {
  try {
    const result = await api.copyCode(account.name);
    showToast(`Код скопирован, действует ${result.remaining} ${plural(result.remaining, "секунду", "секунды", "секунд")}`);
  } catch (err) {
    showToast(`Не удалось скопировать код: ${errorText(err)}`, "error");
  }
}

function renderAccount(account) {
  const item = document.createElement("article");
  item.className = "account-item";
  if (account.favorite) item.classList.add("favorite");

  const main = document.createElement("div");
  main.className = "account-main";

  const name = document.createElement("div");
  name.className = "account-name";
  name.textContent = account.name;
  main.appendChild(name);

  if (account.description) {
    const description = document.createElement("div");
    description.className = "account-description";
    description.textContent = account.description;
    description.title = account.description;
    main.appendChild(description);
  }

  const actions = document.createElement("div");
  actions.className = "account-actions";

  const star = document.createElement("button");
  star.type = "button";
  star.className = `icon-btn star-btn${account.favorite ? " active" : ""}`;
  star.textContent = account.favorite ? "★" : "☆";
  star.title = account.favorite ? "Убрать из избранного" : "Добавить в избранное";
  star.setAttribute("aria-label", star.title);
  star.addEventListener("click", async () => {
    setBusy(star, true, "…");
    try {
      await api.setFavorite(account.name, !account.favorite);
      await loadAccounts();
      await api.refreshTray();
    } catch (err) {
      showToast(`Не удалось изменить избранное: ${errorText(err)}`, "error");
    } finally {
      setBusy(star, false);
    }
  });

  const copy = document.createElement("button");
  copy.type = "button";
  copy.className = "icon-btn action-btn copy-btn";
  copy.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="8" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h9"/></svg>';
  copy.title = "Скопировать код";
  copy.setAttribute("aria-label", "Скопировать код");
  copy.dataset.tip = "Скопировать код";
  copy.addEventListener("click", async () => {
    copy.disabled = true;
    try {
      await copyAccount(account);
    } finally {
      copy.disabled = false;
    }
  });

  const edit = document.createElement("button");
  edit.type = "button";
  edit.className = "icon-btn action-btn edit-btn";
  edit.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4L18.5 9.5a2.1 2.1 0 0 0-3-3L5 17v3Z"/><path d="m13.5 6.5 4 4"/></svg>';
  edit.title = "Изменить аккаунт";
  edit.setAttribute("aria-label", "Изменить аккаунт");
  edit.dataset.tip = "Изменить аккаунт";
  edit.addEventListener("click", () => openAccountDialog(account));

  actions.append(star, copy, edit);
  item.append(main, actions);
  item.addEventListener("click", (event) => {
    if (event.target.closest("button")) return;
    copyAccount(account);
  });
  return item;
}

function openAccountDialog(account = null) {
  const dialog = $("accountDialog");
  if (!dialog) return;
  state.editing = account?.name || "";
  $("accountDialogTitle").textContent = account ? "Изменить аккаунт" : "Новый аккаунт";
  $("accountOriginalName").value = account?.name || "";
  $("accountName").value = account?.name || "";
  $("accountDescription").value = account?.description || "";
  $("accountFavorite").checked = Boolean(account?.favorite);
  $("accountSecret").value = "";
  $("accountSecretLabel").textContent = account ? "Новый секрет (необязательно)" : "Секрет (Base32)";
  $("accountSecretHint").textContent = account
    ? "Оставьте пустым, чтобы сохранить текущий секрет. Текущий секрет не показывается."
    : "Вставьте Base32-секрет из сервиса.";
  $("accountDeleteBtn").hidden = !account;
  $("accountDeleteBtn").classList.toggle("hidden", !account);
  openDialog(dialog);
  setTimeout(() => $("accountName")?.focus(), 50);
  return dialog;
}

async function saveAccount(event) {
  event.preventDefault();
  const button = $("accountSaveBtn");
  const input = {
    originalName: $("accountOriginalName").value,
    name: $("accountName").value.trim(),
    description: $("accountDescription").value,
    favorite: $("accountFavorite").checked,
    newSecret: $("accountSecret").value,
  };
  if (!input.name) {
    showToast("Укажите имя аккаунта.", "error");
    return;
  }
  if (!input.originalName && !input.newSecret.trim()) {
    showToast("Укажите секрет аккаунта.", "error");
    return;
  }
  setBusy(button, true, "Сохранение…");
  try {
    await api.saveAccount(input);
    closeDialog($("accountDialog"));
    showToast(input.originalName ? "Аккаунт обновлён." : "Аккаунт добавлен.");
    await loadAccounts();
    await api.refreshTray();
  } catch (err) {
    showToast(`Не удалось сохранить: ${errorText(err)}`, "error");
  } finally {
    setBusy(button, false);
  }
}

function askConfirm(title, message) {
  return new Promise((resolve) => {
    const dialog = $("confirmDialog");
    if (!dialog) {
      resolve(window.confirm(message));
      return;
    }
    $("confirmTitle").textContent = title;
    $("confirmMessage").textContent = message;
    let settled = false;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      $("confirmOkBtn").onclick = null;
      $("confirmCancelBtn").onclick = null;
      closeDialog(dialog);
      resolve(value);
    };
    $("confirmOkBtn").onclick = () => finish(true);
    $("confirmCancelBtn").onclick = () => finish(false);
    openDialog(dialog);
  });
}

async function deleteAccount() {
  const name = $("accountOriginalName").value;
  if (!name) return;
  const ok = await askConfirm("Удалить аккаунт?", `Аккаунт «${name}» будет удалён без возможности отмены.`);
  if (!ok) return;
  const button = $("accountDeleteBtn");
  setBusy(button, true, "Удаление…");
  try {
    await api.deleteAccount(name);
    closeDialog($("accountDialog"));
    showToast("Аккаунт удалён.");
    await loadAccounts();
    await api.refreshTray();
  } catch (err) {
    showToast(`Не удалось удалить: ${errorText(err)}`, "error");
  } finally {
    setBusy(button, false);
  }
}

async function lockVault() {
  try {
    state.status = await api.lock();
    closeDialog($("settingsDialog"));
    closeDialog($("accountDialog"));
    updateUnlockScreen();
    setScreen("unlock");
    await api.refreshTray();
    showToast("Хранилище заблокировано.");
  } catch (err) {
    showToast(`Не удалось заблокировать: ${errorText(err)}`, "error");
  }
}

function openSettings() {
  const status = state.status || {};
  $("settingsPath").textContent = status.path || "";
  $("settingsProvider").textContent = status.provider || "—";
  $("settingsVersion").textContent = $("versionInfo")?.textContent || "версия dev";
  $("themeSelect").value = currentTheme();
  updateThemeModeControl(currentThemeMode());
  const target = $("rekeyTarget");
  const keyringOption = [...target.options].find((option) => option.value === "keyring");
  if (keyringOption) keyringOption.disabled = !status.keyringAvailable;
  target.value = status.provider === "keyring" && status.keyringAvailable ? "passphrase" : "keyring";
  $("rekeyPassphrase").value = "";
  $("exportPassphrase").value = "";
  $("exportPassphrase2").value = "";
  $("importPassphrase").value = "";
  openDialog($("settingsDialog"));
}

async function doRekey() {
  const target = $("rekeyTarget").value;
  const passphrase = $("rekeyPassphrase").value;
  if (target === "passphrase" && !passphrase) {
    showToast("Для режима «Пароль» нужен новый пароль.", "error");
    return;
  }
  const button = $("rekeyBtn");
  setBusy(button, true, "Перешифровка…");
  try {
    state.status = await api.rekey(target, passphrase);
    $("rekeyPassphrase").value = "";
    $("settingsProvider").textContent = state.status.provider;
    await api.refreshTray();
    showToast(`Провайдер изменён на «${state.status.provider}».`);
  } catch (err) {
    showToast(`Не удалось сменить провайдер: ${errorText(err)}`, "error");
  } finally {
    setBusy(button, false);
  }
}

async function doExport() {
  const passphrase = $("exportPassphrase").value;
  const confirmation = $("exportPassphrase2").value;
  if (!passphrase) {
    showToast("Введите пароль для резервной копии.", "error");
    return;
  }
  if (passphrase !== confirmation) {
    showToast("Пароли резервной копии не совпадают.", "error");
    return;
  }
  const button = $("exportBtn");
  setBusy(button, true, "Экспорт…");
  try {
    const path = await api.saveExport(passphrase);
    if (!path) {
      showToast("Экспорт отменён.");
    } else {
      showToast(`Резервная копия сохранена: ${path}`);
      $("exportPassphrase").value = "";
      $("exportPassphrase2").value = "";
    }
  } catch (err) {
    showToast(`Не удалось экспортировать: ${errorText(err)}`, "error");
  } finally {
    setBusy(button, false);
  }
}

async function chooseImport() {
  const passphrase = $("importPassphrase").value;
  if (!passphrase) {
    showToast("Введите пароль резервной копии.", "error");
    return;
  }
  const button = $("importBtn");
  setBusy(button, true, "Чтение…");
  try {
    const selection = await api.chooseImport(passphrase);
    if (!selection?.path) {
      showToast("Импорт отменён.");
      return;
    }
    state.importSelection = selection;
    state.importPassphrase = passphrase;
    renderImportPreview(selection.preview);
    openDialog($("importDialog"));
  } catch (err) {
    showToast(`Не удалось прочитать резервную копию: ${errorText(err)}`, "error");
  } finally {
    setBusy(button, false);
  }
}

function renderImportPreview(preview) {
  if (!preview) return;
  $("importSummary").textContent = `Всего: ${preview.total}. Новых: ${preview.new?.length || 0}. Конфликтов: ${preview.conflicts?.length || 0}.`;
  const list = $("importList");
  list.replaceChildren();
  const conflicts = new Set(preview.conflicts || []);
  for (const account of preview.accounts || []) {
    const row = document.createElement("div");
    row.className = "import-row";
    const name = document.createElement("span");
    name.textContent = account.name;
    const badge = document.createElement("span");
    const conflict = conflicts.has(account.name);
    badge.className = `badge ${conflict ? "badge-warn" : "badge-ok"}`;
    badge.textContent = conflict ? "конфликт" : "новый";
    row.append(name, badge);
    list.appendChild(row);
  }
}

async function applyImport() {
  const selection = state.importSelection;
  if (!selection?.path) return;
  const button = $("importApplyBtn");
  setBusy(button, true, "Импорт…");
  try {
    const stats = await api.applyImport(selection.path, state.importPassphrase, $("importOverwrite").checked);
    closeDialog($("importDialog"));
    $("importPassphrase").value = "";
    state.importSelection = null;
    state.importPassphrase = "";
    showToast(`Импорт завершён: добавлено ${stats.added}, обновлено ${stats.updated}, пропущено ${stats.skipped}.`);
    await loadAccounts();
    await api.refreshTray();
  } catch (err) {
    showToast(`Не удалось импортировать: ${errorText(err)}`, "error");
  } finally {
    setBusy(button, false);
  }
}

function wireEvents() {
  $("createProvider")?.addEventListener("change", updateCreateFields);

  $("createForm")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const provider = $("createProvider").value;
    const passphrase = $("createPassphrase").value;
    if (provider === "passphrase") {
      if (!passphrase) {
        showToast("Введите пароль vault.", "error");
        return;
      }
      if (passphrase !== $("createPassphrase2").value) {
        showToast("Пароли не совпадают.", "error");
        return;
      }
    }
    const button = $("createBtn");
    setBusy(button, true, "Создание…");
    try {
      state.status = await api.create(provider, passphrase);
      $("createPassphrase").value = "";
      $("createPassphrase2").value = "";
      await refreshStatus();
      await api.refreshTray();
      showToast("Хранилище создано.");
    } catch (err) {
      showToast(`Не удалось создать хранилище: ${errorText(err)}`, "error");
    } finally {
      setBusy(button, false);
    }
  });

  $("unlockForm")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const passphrase = $("unlockPassphrase").value;
    const button = $("unlockBtn");
    setBusy(button, true, "Разблокировка…");
    try {
      state.status = await api.unlock(passphrase);
      $("unlockPassphrase").value = "";
      await refreshStatus();
      await api.refreshTray();
      showToast("Хранилище разблокировано.");
    } catch (err) {
      const hint = $("unlockHint");
      if (hint) {
        hint.textContent = errorText(err);
        hint.className = "form-hint error";
      }
      showToast(`Не удалось разблокировать: ${errorText(err)}`, "error");
    } finally {
      setBusy(button, false);
    }
  });

  $("titlebarCloseBtn")?.addEventListener("click", closeWindow);
  $("lockBtn")?.addEventListener("click", lockVault);
  $("settingsBtn")?.addEventListener("click", openSettings);
  $("settingsCloseBtn")?.addEventListener("click", () => closeDialog($("settingsDialog")));
  $("themeSelect")?.addEventListener("change", (event) => applyTheme(event.target.value, currentThemeMode()));
  $("themeModeBtn")?.addEventListener("click", cycleThemeMode);
  $("addBtn")?.addEventListener("click", () => openAccountDialog());
  $("emptyAddBtn")?.addEventListener("click", () => openAccountDialog());
  $("accountForm")?.addEventListener("submit", saveAccount);
  $("accountCancelBtn")?.addEventListener("click", () => closeDialog($("accountDialog")));
  $("accountDeleteBtn")?.addEventListener("click", deleteAccount);
  $("search")?.addEventListener("input", (event) => {
    state.search = event.target.value;
    renderAccounts();
  });
  $("rekeyBtn")?.addEventListener("click", doRekey);
  $("exportBtn")?.addEventListener("click", doExport);
  $("importBtn")?.addEventListener("click", chooseImport);
  $("importApplyBtn")?.addEventListener("click", applyImport);
  $("importCancelBtn")?.addEventListener("click", () => closeDialog($("importDialog")));

  document.querySelectorAll("dialog").forEach((dialog) => {
    dialog.addEventListener("click", (event) => {
      if (event.target === dialog) closeDialog(dialog);
    });
    dialog.addEventListener("close", () => {
      const toast = $("toast");
      if (!toast || !dialog.contains(toast)) return;
      document.body.appendChild(toast);
      toast.className = "toast";
      clearTimeout(state.toastTimer);
    });
  });

  window.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    // Let an open dialog handle Escape first (native dialog cancel).
    if (document.querySelector("dialog[open]")) return;
    event.preventDefault();
    closeWindow();
  });

  Events.On("vault-changed", async () => {
    await refreshStatus();
    await api.refreshTray();
  });
}

function disableInputHints() {
  document.querySelectorAll("input, textarea").forEach((el) => {
    el.setAttribute("autocomplete", el.type === "password" ? "new-password" : "off");
    el.setAttribute("autocorrect", "off");
    el.setAttribute("autocapitalize", "off");
    el.setAttribute("spellcheck", "false");
    el.removeAttribute("title");
  });
}

async function init() {
  window.__2faBooted = true;
  const themeSettings = loadThemeSettings();
  applyTheme(themeSettings.theme, themeSettings.mode, !themeSettings.stored);
  buildRing();
  setInterval(updateRing, 250);
  wireEvents();
  await refreshStatus(true);
}

disableInputHints();
init().catch((err) => {
  window.__2faBooted = true;
  showToast(`Ошибка запуска интерфейса: ${errorText(err)}`, "error");
  setScreen("unlock");
});
