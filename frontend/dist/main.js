// Furor Davidis UI logic
// Scrutator frustra laborat

let profile = null;
let coverLists = [];
let activeCoverListId = '';
let promptEditorUnlocked = false;
const aiLogLines = [];
const MAX_LOG = 300;
let uiLang = localStorage.getItem('furor_ui_lang') || 'en';

const I18N = {
  en: {
    tab_deploy: 'Deploy',
    tab_connect: 'Connect',
    tab_cover: 'AI Cover',
    tab_memory: 'Memory',
    tab_diag: 'Diagnostics',
    tab_settings: 'Settings',
    disconnected: 'Disconnected',
    connected: 'Connected',
    awg_ok: 'AWG OK',
    ai_stopped: 'AI',
    ai_active: 'AI',
    ai_ready: 'Ready',
    ai_not_running: 'Not running',
    decoy_prefix: 'decoy',
    min_short: 'min',
    deploy_confirm: 'Start VPS deploy? This can take 3-7 minutes.',
    deploy_first_cover_check: 'Before first server deploy, review AI Cover sites. Open AI Cover now?',
    deploy_starting: 'Starting deploy...',
    deploy_checking: 'Checking server services...',
    no_active_cover_list: 'No active cover list',
    cover_sites_saved: 'Cover sites saved',
    clear_memory_confirm: 'Clear all memory entries?',
    list_name_empty: 'List name is empty',
    list_required: 'At least one cover list is required',
    delete_list_confirm: 'Delete list "{name}"?',
    select_profile_error: 'Select profile',
    create_profile_error: 'Create profile',
    delete_profile_error: 'Delete profile',
    export_profile_error: 'Export profile',
    import_profile_error: 'Import profile',
    delete_profile_confirm: 'Delete active profile?',
    settings_saved: 'Settings saved',
    error_prefix: 'Error',
    connect_prefix: 'Connect',
    connect_button_idle: 'Libertad',
    connect_button_active: 'Libero',
    hotswap_prefix: 'HotSwap',
    start_prefix: 'Start',
    run_deploy_hint: 'Run Deploy first in the Deploy tab.',
    deploy_done_hint: 'Deploy complete. Click Connect to start the tunnel.',
    records: 'Records',
    success: 'Success',
    failure: 'Failure',
    timeout: 'Timeouts',
    avg_score: 'Avg Score',
    days: 'Days',
    auto_not_selected: '(auto / not selected)',
    ttl_vps_connection: 'VPS connection',
    lbl_host: 'Host / IP',
    lbl_port: 'SSH Port',
    lbl_user: 'User',
    lbl_pass: 'Password',
    ttl_decoy_domains: 'Decoy domains (xray facade)',
    btn_add_domain: '+ Add domain',
    hint_decoys: 'The first domain is used during deploy. HotSwap rotates this list.',
    btn_start_deploy: '¡Viva la libertad',
    btn_verify_server: 'Verify server',
    ttl_deploy_log: 'Deploy log',
    btn_clear: 'clear',
    ttl_awg_tunnel: 'AWG tunnel',
    ttl_hotswap: 'HotSwap decoy domain',
    btn_hotswap_switch: 'Switch',
    hint_hotswap: 'Changes the active xray decoy domain without dropping AWG.',
    ttl_connection_status: 'Connection status',
    k_awg_port: 'AWG port',
    k_active_decoy: 'Active decoy',
    k_physical_ip: 'Physical IP',
    ttl_ai_orchestrator: 'AI orchestrator',
    btn_start: 'Start',
    btn_stop: 'Stop',
    k_last_action: 'Last action',
    k_session: 'Session',
    k_ai_model: 'AI model',
    ttl_adaptive_live: 'Adaptive live',
    k_ad_enabled: 'Enabled',
    k_ad_mode: 'Mode',
    k_ad_session: 'Session (min)',
    k_ad_url: 'URL count',
    k_ad_depth: 'Chain depth',
    k_ad_sitecap: 'Site cap',
    k_ad_rag: 'RAG weight',
    k_ad_policy: 'Timeout policy',
    ttl_cover_sites: 'Cover Sites',
    btn_cover_add: '+ Add',
    lbl_cover_list: 'Cover list',
    lbl_list_name: 'List name',
    btn_create_list: 'Create list',
    btn_rename: 'Rename',
    btn_delete_list: 'Delete list',
    lbl_intensity: 'Intensity',
    intensity_low: 'Low',
    intensity_medium: 'Medium',
    intensity_high: 'High',
    lbl_session_len: 'Session length (min)',
    btn_save: 'Save',
    hint_cover: 'Cover traffic goes through the physical adapter, bypassing AWG. Uses uTLS fingerprints for realistic web traffic.',
    ttl_ai_log: 'AI log',
    ttl_ai_prompt: 'AI prompt',
    btn_prompt_open: 'View prompt',
    btn_prompt_edit: 'Edit',
    btn_prompt_save: 'Save',
    btn_prompt_reset: 'Reset to recommended',
    btn_prompt_close: 'Close',
    hint_prompt_limit: 'System prompt is capped at 700 characters to keep requests compact for small models.',
    warn_prompt_view: 'Warning: editing AI prompt can break protection logic. Open in view-only mode?',
    warn_prompt_edit: 'You are taking full responsibility for prompt changes. This is not recommended. Continue to edit?',
    warn_prompt_reset: 'Reset prompt to the recommended default?',
    prompt_saved: 'Prompt saved',
    prompt_too_long: 'Prompt is too long. Maximum 700 characters.',
    prompt_empty: 'Prompt cannot be empty. Use Reset to recommended if needed.',
    prompt_too_short: 'Prompt is too short. Minimum 40 characters.',
    ttl_rag_memory: 'RAG memory',
    btn_export: 'Export',
    btn_import: 'Import',
    btn_clear_memory: 'Clear',
    hint_memory: 'Stores patterns that worked in your network and injects them into AI prompts. File: <code>furor_memory.json</code>.',
    ttl_top_entries: 'Top entries',
    ttl_local_files: 'Local files',
    ttl_server: 'Server',
    btn_run_diagnostics: 'Run diagnostics',
    ttl_requirements: 'Requirements',
    hint_requirements: 'Install <strong>LM Studio</strong> and start <strong>Local Server</strong>.<br/>Put AWG files next to <code>FurorDavidis.exe</code>:<br/><pre class="pre">FurorDavidis.exe\nawg\\\n    amneziawg.exe\n    wintun.dll</pre>',
    btn_open_lmstudio: 'Open LM Studio',
    ttl_ui_language: 'Interface language',
    lbl_ui_language: 'Language',
    ttl_profiles: 'Profiles',
    lbl_active_profile: 'Active profile',
    lbl_profile_name: 'Profile name',
    lbl_active_client: 'Active client',
    lbl_client_name: 'Client name',
    btn_create: 'Create',
    btn_delete: 'Delete',
    btn_create_server: 'Create Server',
    btn_delete_server: 'Delete Server',
    btn_create_client: 'Create Client',
    btn_delete_client: 'Delete Client',
    btn_refresh: 'Refresh',
    hint_profiles: 'Each server has its own VPS config, and each server can contain multiple client profiles. Switching server/client stops AI and disconnects AWG.',
    ttl_ai_backend: 'AI backend (LM Studio)',
    lbl_model_override: 'Model override (optional)',
    lbl_models_loaded: 'Models loaded in LM Studio',
    lbl_adaptive_enable: 'Enable adaptive control',
    lbl_adaptive_mode: 'Adaptive mode',
    adaptive_mode_conservative: 'Conservative',
    adaptive_mode_balanced: 'Balanced',
    adaptive_mode_aggressive: 'Aggressive',
    lbl_rag_timeout_policy: 'RAG timeout penalty',
    rag_timeout_low: 'Low (0.10)',
    rag_timeout_base: 'Base (0.15)',
    rag_timeout_high: 'High (0.20)',
    btn_refresh_models: 'Refresh model list',
    hint_ai_backend: 'Open LM Studio -> Local Server -> load model -> Start Server.<br/>Recommended: Qwen3-1.7B or any instruct model.<br/>If your PC is low on compute, consider 0.6B models.',
    ttl_awg: 'AWG',
    lbl_awg_path: 'Path to amneziawg.exe',
    lbl_awg_iface: 'Interface name',
    ttl_hotswap_settings: 'HotSwap',
    lbl_hs_enable: 'Enable automatic domain rotation',
    lbl_hs_interval: 'Interval (min)',
    btn_save_settings: 'Save settings'
  },
  ru: {
    tab_deploy: 'Деплой',
    tab_connect: 'Подключение',
    tab_cover: 'AI Cover',
    tab_memory: 'Память',
    tab_diag: 'Диагностика',
    tab_settings: 'Настройки',
    disconnected: 'Отключено',
    connected: 'Подключено',
    awg_ok: 'AWG OK',
    ai_stopped: 'AI',
    ai_active: 'AI',
    ai_ready: 'Готова',
    ai_not_running: 'Не запущена',
    decoy_prefix: 'декой',
    min_short: 'мин',
    deploy_confirm: 'Запустить деплой на VPS? Это займет 3-7 минут.',
    deploy_first_cover_check: 'Перед первым деплоем сервера проверь список сайтов AI Cover. Открыть AI Cover сейчас?',
    deploy_starting: 'Запуск деплоя...',
    deploy_checking: 'Проверка сервисов сервера...',
    no_active_cover_list: 'Нет активного списка cover',
    cover_sites_saved: 'Списки cover сохранены',
    clear_memory_confirm: 'Очистить всю память?',
    list_name_empty: 'Имя списка пустое',
    list_required: 'Должен остаться хотя бы один список',
    delete_list_confirm: 'Удалить список "{name}"?',
    select_profile_error: 'Выбор профиля',
    create_profile_error: 'Создание профиля',
    delete_profile_error: 'Удаление профиля',
    export_profile_error: 'Экспорт профиля',
    import_profile_error: 'Импорт профиля',
    delete_profile_confirm: 'Удалить активный профиль?',
    settings_saved: 'Настройки сохранены',
    error_prefix: 'Ошибка',
    connect_prefix: 'Подключение',
    connect_button_idle: 'Libertad',
    connect_button_active: 'Libero',
    hotswap_prefix: 'HotSwap',
    start_prefix: 'Старт',
    run_deploy_hint: 'Сначала выполните Deploy на вкладке Deploy.',
    deploy_done_hint: 'Деплой завершен. Нажмите Connect для подключения туннеля.',
    records: 'Записей',
    success: 'Успешных',
    failure: 'Неудачных',
    timeout: 'Таймаутов',
    avg_score: 'Средний score',
    days: 'Дней',
    auto_not_selected: '(авто / не выбрано)',
    ttl_vps_connection: 'Подключение к VPS',
    lbl_host: 'Host / IP',
    lbl_port: 'SSH порт',
    lbl_user: 'Пользователь',
    lbl_pass: 'Пароль',
    ttl_decoy_domains: 'Декой-домены (xray фасад)',
    btn_add_domain: '+ Добавить домен',
    hint_decoys: 'Первый домен используется при деплое. HotSwap ротирует список.',
    btn_start_deploy: '¡Viva la libertad',
    btn_verify_server: 'Проверить сервер',
    ttl_deploy_log: 'Лог деплоя',
    btn_clear: 'очистить',
    ttl_awg_tunnel: 'AWG туннель',
    ttl_hotswap: 'HotSwap декой-домена',
    btn_hotswap_switch: 'Сменить',
    hint_hotswap: 'Меняет активный декой-домен xray без разрыва AWG.',
    ttl_connection_status: 'Статус подключения',
    k_awg_port: 'AWG порт',
    k_active_decoy: 'Активный декой',
    k_physical_ip: 'Физический IP',
    ttl_ai_orchestrator: 'AI оркестратор',
    btn_start: 'Запустить',
    btn_stop: 'Стоп',
    k_last_action: 'Последнее действие',
    k_session: 'Сессия',
    k_ai_model: 'AI модель',
    ttl_adaptive_live: 'Адаптив live',
    k_ad_enabled: 'Включен',
    k_ad_mode: 'Режим',
    k_ad_session: 'Сессия (мин)',
    k_ad_url: 'URL количество',
    k_ad_depth: 'Глубина цепочки',
    k_ad_sitecap: 'Лимит сайтов',
    k_ad_rag: 'Вес RAG',
    k_ad_policy: 'Политика timeout',
    ttl_cover_sites: 'Cover-сайты',
    btn_cover_add: '+ Добавить',
    lbl_cover_list: 'Cover-список',
    lbl_list_name: 'Имя списка',
    btn_create_list: 'Создать список',
    btn_rename: 'Переименовать',
    btn_delete_list: 'Удалить список',
    lbl_intensity: 'Интенсивность',
    intensity_low: 'Низкая',
    intensity_medium: 'Средняя',
    intensity_high: 'Высокая',
    lbl_session_len: 'Длительность сессии (мин)',
    btn_save: 'Сохранить',
    hint_cover: 'Cover-трафик идет через физический адаптер в обход AWG. Для реалистичного поведения используется uTLS.',
    ttl_ai_log: 'Лог AI',
    ttl_ai_prompt: 'Промпт AI',
    btn_prompt_open: 'Просмотр промпта',
    btn_prompt_edit: 'Редактировать',
    btn_prompt_save: 'Сохранить',
    btn_prompt_reset: 'Reset to recommended',
    btn_prompt_close: 'Закрыть',
    hint_prompt_limit: 'Системный промпт ограничен 700 символами, чтобы не раздувать запрос для малых моделей.',
    warn_prompt_view: 'Внимание: редактирование AI-промпта может сломать логику защиты. Открыть в режиме просмотра?',
    warn_prompt_edit: 'Вы берете полную ответственность за изменения промпта. Это не рекомендуется. Продолжить редактирование?',
    warn_prompt_reset: 'Сбросить промпт на рекомендованный по умолчанию?',
    prompt_saved: 'Промпт сохранен',
    prompt_too_long: 'Промпт слишком длинный. Максимум 700 символов.',
    prompt_empty: 'Промпт не может быть пустым. При необходимости используйте Reset to recommended.',
    prompt_too_short: 'Промпт слишком короткий. Минимум 40 символов.',
    ttl_rag_memory: 'RAG память',
    btn_export: 'Экспорт',
    btn_import: 'Импорт',
    btn_clear_memory: 'Очистить',
    hint_memory: 'Копит паттерны, которые сработали в вашей сети, и добавляет их в AI промпт. Файл: <code>furor_memory.json</code>.',
    ttl_top_entries: 'Топ записей',
    ttl_local_files: 'Локальные файлы',
    ttl_server: 'Сервер',
    btn_run_diagnostics: 'Запустить диагностику',
    ttl_requirements: 'Требования',
    hint_requirements: 'Установите <strong>LM Studio</strong> и запустите <strong>Local Server</strong>.<br/>Положите файлы AWG рядом с <code>FurorDavidis.exe</code>:<br/><pre class="pre">FurorDavidis.exe\nawg\\\n    amneziawg.exe\n    wintun.dll</pre>',
    btn_open_lmstudio: 'Открыть LM Studio',
    ttl_ui_language: 'Язык интерфейса',
    lbl_ui_language: 'Язык',
    ttl_profiles: 'Профили',
    lbl_active_profile: 'Активный профиль',
    lbl_profile_name: 'Имя профиля',
    lbl_active_client: 'Активный клиент',
    lbl_client_name: 'Имя клиента',
    btn_create: 'Создать',
    btn_delete: 'Удалить',
    btn_create_server: 'Создать сервер',
    btn_delete_server: 'Удалить сервер',
    btn_create_client: 'Создать клиент',
    btn_delete_client: 'Удалить клиент',
    btn_refresh: 'Обновить',
    hint_profiles: 'У каждого сервера своя VPS-конфигурация, и внутри сервера может быть несколько клиентских профилей. При переключении сервера/клиента AI останавливается и AWG отключается.',
    ttl_ai_backend: 'AI бэкенд (LM Studio)',
    lbl_model_override: 'Модель (необязательно)',
    lbl_models_loaded: 'Модели в LM Studio',
    lbl_adaptive_enable: 'Включить адаптивное управление',
    lbl_adaptive_mode: 'Адаптивный режим',
    adaptive_mode_conservative: 'Консервативный',
    adaptive_mode_balanced: 'Сбалансированный',
    adaptive_mode_aggressive: 'Агрессивный',
    lbl_rag_timeout_policy: 'Штраф RAG за таймаут',
    rag_timeout_low: 'Низкий (0.10)',
    rag_timeout_base: 'Базовый (0.15)',
    rag_timeout_high: 'Высокий (0.20)',
    btn_refresh_models: 'Обновить список моделей',
    hint_ai_backend: 'Откройте LM Studio -> Local Server -> загрузите модель -> Start Server.<br/>Рекомендуется: Qwen3-1.7B или любая instruct-модель.<br/>Если ресурсов мало, рассмотрите модели 0.6B.',
    ttl_awg: 'AWG',
    lbl_awg_path: 'Путь к amneziawg.exe',
    lbl_awg_iface: 'Имя интерфейса',
    ttl_hotswap_settings: 'HotSwap',
    lbl_hs_enable: 'Автоматическая ротация домена',
    lbl_hs_interval: 'Интервал (мин)',
    btn_save_settings: 'Сохранить настройки'
  }
};

window.addEventListener('DOMContentLoaded', async () => {
  if (!I18N[uiLang]) uiLang = 'en';
  const langTop = document.getElementById('lang-select');
  const langSettings = document.getElementById('s-lang-select');
  if (langTop) langTop.value = uiLang;
  if (langSettings) langSettings.value = uiLang;
  applyLanguage();
  syncTrayLanguage();

  document.querySelectorAll('.tab').forEach(t =>
    t.addEventListener('click', () => switchTab(t.dataset.tab, t))
  );

  window.runtime.EventsOn('log', e => addLog('ai-log', e));
  window.runtime.EventsOn('deploy_log', ln => appendDeployLog(ln));
  window.runtime.EventsOn('deploy_done', port => {
    appendDeployLog('Deploy complete, AWG port: ' + port);
    enableDeployBtn(true);
  });
  window.runtime.EventsOn('connected', v => setConnected(v));

  await loadProfile();
  await refreshServersAndClients();
  await refreshLMStudioModels();
  await refreshStatus();
  await refreshMemory();
  setInterval(refreshStatus, 5000);
  setInterval(refreshMemory, 30000);
});

function t(key, vars = {}) {
  const dict = I18N[uiLang] || I18N.en;
  let text = dict[key] || I18N.en[key] || key;
  Object.keys(vars).forEach(k => {
    text = text.replaceAll(`{${k}}`, String(vars[k]));
  });
  return text;
}

function setLanguage(lang) {
  uiLang = I18N[lang] ? lang : 'en';
  localStorage.setItem('furor_ui_lang', uiLang);
  const langTop = document.getElementById('lang-select');
  const langSettings = document.getElementById('s-lang-select');
  if (langTop && langTop.value !== uiLang) langTop.value = uiLang;
  if (langSettings && langSettings.value !== uiLang) langSettings.value = uiLang;
  applyLanguage();
  syncTrayLanguage();
  refreshStatus();
  refreshMemory();
}

async function syncTrayLanguage() {
  try {
    await window.go.main.App.SetUILanguage(uiLang);
  } catch (_) {}
}

function applyLanguage() {
  document.documentElement.lang = uiLang;
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    if (key) el.textContent = t(key);
  });

  const mapText = {
    'ttl-vps-connection': 'ttl_vps_connection',
    'lbl-host': 'lbl_host',
    'lbl-port': 'lbl_port',
    'lbl-user': 'lbl_user',
    'lbl-pass': 'lbl_pass',
    'ttl-decoy-domains': 'ttl_decoy_domains',
    'btn-add-domain': 'btn_add_domain',
    'hint-decoys': 'hint_decoys',
    'btn-start-deploy': 'btn_start_deploy',
    'btn-verify-server': 'btn_verify_server',
    'ttl-deploy-log': 'ttl_deploy_log',
    'btn-clear-deploy-log': 'btn_clear',
    'ttl-awg-tunnel': 'ttl_awg_tunnel',
    'ttl-hotswap': 'ttl_hotswap',
    'btn-hotswap-switch': 'btn_hotswap_switch',
    'hint-hotswap': 'hint_hotswap',
    'ttl-connection-status': 'ttl_connection_status',
    'k-awg-port': 'k_awg_port',
    'k-active-decoy': 'k_active_decoy',
    'k-physical-ip': 'k_physical_ip',
    'ttl-ai-orchestrator': 'ttl_ai_orchestrator',
    'btn-ai-start': 'btn_start',
    'btn-ai-stop': 'btn_stop',
    'k-last-action': 'k_last_action',
    'k-session': 'k_session',
    'k-ai-model': 'k_ai_model',
    'ttl-adaptive-live': 'ttl_adaptive_live',
    'k-ad-enabled': 'k_ad_enabled',
    'k-ad-mode': 'k_ad_mode',
    'k-ad-session': 'k_ad_session',
    'k-ad-url': 'k_ad_url',
    'k-ad-depth': 'k_ad_depth',
    'k-ad-sitecap': 'k_ad_sitecap',
    'k-ad-rag': 'k_ad_rag',
    'k-ad-policy': 'k_ad_policy',
    'ttl-cover-sites': 'ttl_cover_sites',
    'btn-cover-add': 'btn_cover_add',
    'lbl-cover-list': 'lbl_cover_list',
    'lbl-list-name': 'lbl_list_name',
    'btn-create-list': 'btn_create_list',
    'btn-rename-list': 'btn_rename',
    'btn-delete-list': 'btn_delete_list',
    'lbl-intensity': 'lbl_intensity',
    'lbl-session-len': 'lbl_session_len',
    'btn-save-cover': 'btn_save',
    'hint-cover': 'hint_cover',
    'ttl-ai-prompt': 'ttl_ai_prompt',
    'btn-prompt-open': 'btn_prompt_open',
    'btn-prompt-edit': 'btn_prompt_edit',
    'btn-prompt-save': 'btn_prompt_save',
    'btn-prompt-reset': 'btn_prompt_reset',
    'btn-prompt-close': 'btn_prompt_close',
    'ttl-ai-log': 'ttl_ai_log',
    'btn-clear-ai-log': 'btn_clear',
    'ttl-rag-memory': 'ttl_rag_memory',
    'btn-mem-export': 'btn_export',
    'btn-mem-import': 'btn_import',
    'btn-mem-clear': 'btn_clear_memory',
    'ttl-top-entries': 'ttl_top_entries',
    'ttl-local-files': 'ttl_local_files',
    'ttl-server': 'ttl_server',
    'btn-run-diagnostics': 'btn_run_diagnostics',
    'ttl-requirements': 'ttl_requirements',
    'btn-open-lmstudio': 'btn_open_lmstudio',
    'ttl-ui-language': 'ttl_ui_language',
    'lbl-ui-language': 'lbl_ui_language',
    'ttl-profiles': 'ttl_profiles',
    'lbl-active-profile': 'lbl_active_profile',
    'lbl-profile-name': 'lbl_profile_name',
    'lbl-active-client': 'lbl_active_client',
    'lbl-client-name': 'lbl_client_name',
    'btn-profile-create': 'btn_create_server',
    'btn-profile-delete': 'btn_delete_server',
    'btn-profile-export': 'btn_export',
    'btn-profile-import': 'btn_import',
    'btn-client-create': 'btn_create_client',
    'btn-client-delete': 'btn_delete_client',
    'btn-profile-refresh': 'btn_refresh',
    'hint-profiles': 'hint_profiles',
    'ttl-ai-backend': 'ttl_ai_backend',
    'lbl-model-override': 'lbl_model_override',
    'lbl-models-loaded': 'lbl_models_loaded',
    'lbl-adaptive-enable': 'lbl_adaptive_enable',
    'lbl-adaptive-mode': 'lbl_adaptive_mode',
    'lbl-rag-timeout-policy': 'lbl_rag_timeout_policy',
    'btn-refresh-models': 'btn_refresh_models',
    'ttl-awg': 'ttl_awg',
    'lbl-awg-path': 'lbl_awg_path',
    'lbl-awg-iface': 'lbl_awg_iface',
    'ttl-hotswap-settings': 'ttl_hotswap_settings',
    'lbl-hs-enable': 'lbl_hs_enable',
    'lbl-hs-interval': 'lbl_hs_interval',
    'btn-save-settings': 'btn_save_settings'
  };
  Object.entries(mapText).forEach(([id, key]) => {
    const el = document.getElementById(id);
    if (el) el.textContent = t(key);
  });

  const htmlMap = {
    'hint-prompt-limit': 'hint_prompt_limit',
    'hint-memory': 'hint_memory',
    'hint-requirements': 'hint_requirements',
    'hint-ai-backend': 'hint_ai_backend'
  };
  Object.entries(htmlMap).forEach(([id, key]) => {
    const el = document.getElementById(id);
    if (el) el.innerHTML = t(key);
  });

  const placeholders = {
    'cover-list-name': t('lbl_list_name'),
    's-server-name': t('lbl_profile_name'),
    's-client-name': t('lbl_client_name'),
    's-ai-model': t('lbl_model_override')
  };
  Object.entries(placeholders).forEach(([id, text]) => {
    const el = document.getElementById(id);
    if (el) el.placeholder = text;
  });

  const dotAwg = document.getElementById('dot-awg');
  const dotAi = document.getElementById('dot-ai');
  if (dotAwg) dotAwg.title = t('ttl_awg_tunnel');
  if (dotAi) dotAi.title = t('ttl_ai_orchestrator');

  const intensity = document.getElementById('intensity');
  if (intensity) {
    const options = intensity.querySelectorAll('option');
    if (options[0]) options[0].textContent = t('intensity_low');
    if (options[1]) options[1].textContent = t('intensity_medium');
    if (options[2]) options[2].textContent = t('intensity_high');
  }

  const aiModelSel = document.getElementById('s-ai-model-select');
  if (aiModelSel && aiModelSel.options.length > 0 && aiModelSel.options[0].value === '') {
    aiModelSel.options[0].textContent = t('auto_not_selected');
  }

  const ragTimeoutSel = document.getElementById('s-rag-timeout-policy');
  if (ragTimeoutSel) {
    const low = document.getElementById('opt-rag-timeout-low');
    const base = document.getElementById('opt-rag-timeout-base');
    const high = document.getElementById('opt-rag-timeout-high');
    if (low) low.textContent = t('rag_timeout_low');
    if (base) base.textContent = t('rag_timeout_base');
    if (high) high.textContent = t('rag_timeout_high');
  }

  const adaptiveModeSel = document.getElementById('s-adaptive-mode');
  if (adaptiveModeSel) {
    const c = document.getElementById('opt-adaptive-conservative');
    const b = document.getElementById('opt-adaptive-balanced');
    const a = document.getElementById('opt-adaptive-aggressive');
    if (c) c.textContent = t('adaptive_mode_conservative');
    if (b) b.textContent = t('adaptive_mode_balanced');
    if (a) a.textContent = t('adaptive_mode_aggressive');
  }
}

function switchTab(name, btn) {
  document.querySelectorAll('.pane').forEach(p => p.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  document.getElementById('tab-' + name).classList.add('active');
  btn.classList.add('active');
  if (name === 'diag') runDiag();
  if (name === 'memory') refreshMemory();
}

async function refreshStatus() {
  try {
    const s = await window.go.main.App.GetStatus();
    setConnected(s.connected);
    setAIRunning(s.running);
    const btnConnect = document.getElementById('btn-connect');
    const btnDisconnect = document.getElementById('btn-disconnect');
    if (btnConnect) btnConnect.disabled = !!s.connected;
    if (btnDisconnect) btnDisconnect.disabled = !s.connected;

    const aiDot = document.getElementById('dot-ai');
    aiDot.className = s.ai_ready ? 'dot on' : 'dot off';

    setText('txt-decoy', `${t('decoy_prefix')}: ` + (s.active_decoy || '-'));
    setText('ci-physip', s.phys_ip || '-');
    setText('ci-decoy', s.active_decoy || '-');
    setText('ci-port', profile?.awg_listen_port || '-');
    setText('ai-last', s.last_action || '-');
    setText('ai-session', s.session_min + ' ' + t('min_short'));
    setText('ai-model-status', s.ai_ready ? t('ai_ready') : t('ai_not_running'));
    setAdaptiveValue('ad-enabled', s.adaptive_enabled ? 'ON' : 'OFF', s.adaptive_enabled ? 'state-on' : 'state-off');
    setAdaptiveValue('ad-mode', formatAdaptiveMode(s.adaptive_mode), adaptiveModeClass(s.adaptive_mode));
    setAdaptiveValue('ad-session', String(s.adaptive_session_min ?? '-'));
    setAdaptiveValue('ad-url', String(s.adaptive_url_count ?? '-'));
    setAdaptiveValue('ad-depth', String(s.adaptive_chain_depth ?? '-'));
    setAdaptiveValue('ad-sitecap', String(s.adaptive_site_cap ?? '-'));
    setAdaptiveValue(
      'ad-rag',
      typeof s.adaptive_rag_weight === 'number' ? s.adaptive_rag_weight.toFixed(2) : '-',
      adaptiveRagClass(s.adaptive_rag_weight)
    );
    setAdaptiveValue('ad-policy', s.adaptive_timeout_policy || '-', adaptiveTimeoutPolicyClass(s.adaptive_timeout_policy));
  } catch (_) {}
}

function setAdaptiveValue(id, text, stateClass = '') {
  const el = document.getElementById(id);
  if (!el) return;
  el.textContent = text;
  el.className = stateClass ? `v ${stateClass}` : 'v';
}

function adaptiveModeClass(mode) {
  const m = String(mode || '').toLowerCase();
  if (m === 'conservative') return 'state-conservative';
  if (m === 'aggressive') return 'state-aggressive';
  return 'state-balanced';
}

function adaptiveTimeoutPolicyClass(policy) {
  const p = String(policy || '').toLowerCase();
  if (p === 'low') return 'state-timeout-low';
  if (p === 'high') return 'state-timeout-high';
  return 'state-timeout-base';
}

function adaptiveRagClass(weight) {
  const w = Number(weight);
  if (!Number.isFinite(w)) return '';
  if (w >= 0.75) return 'state-rag-high';
  if (w >= 0.5) return 'state-rag-mid';
  return 'state-rag-low';
}

function formatAdaptiveMode(mode) {
  const m = String(mode || '').toLowerCase();
  if (m === 'conservative') return t('adaptive_mode_conservative');
  if (m === 'aggressive') return t('adaptive_mode_aggressive');
  return t('adaptive_mode_balanced');
}

function setConnected(v) {
  const box = document.getElementById('conn-status-box');
  box.className = 'status-box ' + (v ? 'connected' : 'disconnected');
  setText('conn-icon', v ? 'OK' : 'X');
  setText('conn-text', v ? t('connected') : t('disconnected'));
  setText('btn-connect', v ? t('connect_button_active') : t('connect_button_idle'));
  document.getElementById('dot-awg').className = 'dot ' + (v ? 'on' : 'off');
  setText('txt-awg', v ? t('awg_ok') : t('disconnected'));
}

function setAIRunning(v) {
  const box = document.getElementById('ai-status-box');
  box.className = 'status-box ' + (v ? 'connected' : 'disconnected');
  setText('ai-icon', v ? 'OK' : 'X');
  setText('ai-text', v ? t('ai_active') : t('ai_stopped'));
  document.getElementById('btn-ai-start').disabled = v;
  document.getElementById('btn-ai-stop').disabled = !v;
}

async function doDeploy() {
  if (!profile?.deployed) {
    const proceed = confirm(t('deploy_first_cover_check'));
    if (!proceed) {
      const coverBtn = document.querySelector('.tab[data-tab="cover"]');
      if (coverBtn) switchTab('cover', coverBtn);
      return;
    }
  }
  if (!confirm(t('deploy_confirm'))) return;
  await saveSettingsFromDeployTab();
  enableDeployBtn(false);
  clearDeployLog();
  appendDeployLog(t('deploy_starting'));
  try {
    await window.go.main.App.DeployServer();
    await loadProfile();
    populateHotSwap();
  } catch (e) {
    appendDeployLog('ERROR: ' + e);
    enableDeployBtn(true);
  }
}

async function doVerify() {
  clearDeployLog();
  appendDeployLog(t('deploy_checking'));
  try {
    await window.go.main.App.VerifyServer();
  } catch (e) {
    appendDeployLog('ERROR: ' + e);
  }
}

async function saveSettingsFromDeployTab() {
  if (!profile) return;
  syncActiveCoverListSitesFromEditor();
  const active = getActiveCoverList();
  const decoys = extractDomainsFromSites(active?.sites || []);
  const p = {
    ...profile,
    vps_host: val('d-host'),
    vps_port: parseInt(val('d-port')) || 22,
    vps_user: val('d-user'),
    vps_password: val('d-pass') || profile.vps_password,
    decoy_domains: decoys.length ? decoys : profile.decoy_domains,
    active_decoy_domain: decoys[0] || profile.active_decoy_domain || '',
  };
  try {
    await window.go.main.App.SaveProfile(p);
    profile = p;
    renderTagList('decoy-list', p.decoy_domains || []);
    populateHotSwap();
    await refreshServersAndClients();
  } catch (_) {}
}

function appendDeployLog(line) {
  const log = document.getElementById('deploy-log');
  const d = document.createElement('div');
  d.className = 'll ' + (line.startsWith('ERROR') ? 'ERROR' : 'INFO');
  d.innerHTML = `<span class="lt">${now()}</span>${esc(line)}`;
  log.appendChild(d);
  log.scrollTop = log.scrollHeight;
}

function clearDeployLog() {
  document.getElementById('deploy-log').innerHTML = '';
}

function enableDeployBtn(v) {
  document.querySelector('[onclick="doDeploy()"]').disabled = !v;
}

async function doConnect() {
  try {
    await window.go.main.App.ConnectAWG();
  } catch (e) {
    alert(t('connect_prefix') + ': ' + e);
  }
}

async function doDisconnect() {
  await window.go.main.App.DisconnectAWG();
  await refreshStatus();
}

async function doHotSwap() {
  const domain = document.getElementById('hs-domain').value;
  if (!domain) return;
  try {
    await window.go.main.App.HotSwapDomain(domain);
    setText('ci-decoy', domain);
    setText('txt-decoy', `${t('decoy_prefix')}: ` + domain);
  } catch (e) {
    alert(t('hotswap_prefix') + ': ' + e);
  }
}

function populateHotSwap() {
  const sel = document.getElementById('hs-domain');
  sel.innerHTML = '';
  (profile?.decoy_domains || []).forEach(d => {
    const o = document.createElement('option');
    o.value = o.textContent = d;
    sel.appendChild(o);
  });
}

async function doAIStart() {
  try {
    await window.go.main.App.Start();
    setAIRunning(true);
  } catch (e) {
    alert(t('start_prefix') + ': ' + e);
  }
}

async function doAIStop() {
  await window.go.main.App.Stop();
  setAIRunning(false);
}

async function openPromptEditor() {
  if (!confirm(t('warn_prompt_view'))) return;
  try {
    const eff = await window.go.main.App.GetEffectiveAIPrompt();
    setVal('ai-prompt-text', eff || '');
    const ta = document.getElementById('ai-prompt-text');
    const wrap = document.getElementById('prompt-editor-wrap');
    const btnSave = document.getElementById('btn-prompt-save');
    if (ta) ta.readOnly = true;
    if (wrap) wrap.style.display = '';
    if (btnSave) btnSave.disabled = true;
    promptEditorUnlocked = false;
  } catch (e) {
    alert(e);
  }
}

function enablePromptEditing() {
  if (!confirm(t('warn_prompt_edit'))) return;
  const ta = document.getElementById('ai-prompt-text');
  const btnSave = document.getElementById('btn-prompt-save');
  if (ta) {
    ta.readOnly = false;
    ta.focus();
  }
  if (btnSave) btnSave.disabled = false;
  promptEditorUnlocked = true;
}

function closePromptEditor() {
  const wrap = document.getElementById('prompt-editor-wrap');
  const ta = document.getElementById('ai-prompt-text');
  const btnSave = document.getElementById('btn-prompt-save');
  if (wrap) wrap.style.display = 'none';
  if (ta) ta.readOnly = true;
  if (btnSave) btnSave.disabled = true;
  promptEditorUnlocked = false;
}

async function savePromptEditor() {
  if (!profile || !promptEditorUnlocked) return;
  const text = val('ai-prompt-text').trim();
  if (!text) {
    alert(t('prompt_empty'));
    return;
  }
  if ([...text].length < 40) {
    alert(t('prompt_too_short'));
    return;
  }
  if ([...text].length > 700) {
    alert(t('prompt_too_long'));
    return;
  }
  const p = { ...profile, ai_system_prompt: text };
  try {
    await window.go.main.App.SaveProfile(p);
    profile = p;
    alert(t('prompt_saved'));
    closePromptEditor();
  } catch (e) {
    alert(e);
  }
}

async function resetPromptRecommended() {
  if (!profile) return;
  if (!confirm(t('warn_prompt_reset'))) return;
  const p = { ...profile, ai_system_prompt: '' };
  try {
    await window.go.main.App.SaveProfile(p);
    profile = p;
    const eff = await window.go.main.App.GetEffectiveAIPrompt();
    setVal('ai-prompt-text', eff || '');
    const ta = document.getElementById('ai-prompt-text');
    const btnSave = document.getElementById('btn-prompt-save');
    if (ta) ta.readOnly = true;
    if (btnSave) btnSave.disabled = true;
    promptEditorUnlocked = false;
    alert(t('prompt_saved'));
  } catch (e) {
    alert(e);
  }
}

async function saveCover() {
  if (!profile) return;
  syncActiveCoverListSitesFromEditor();
  const active = getActiveCoverList();
  if (!active) {
    alert(t('no_active_cover_list'));
    return;
  }
  const syncedDecoys = extractDomainsFromSites(active.sites || []);
  const p = {
    ...profile,
    cover_lists: coverLists,
    active_cover_list_id: activeCoverListId,
    cover_sites: active.sites || [],
    behavior_profile: active.name || 'Custom',
    decoy_domains: syncedDecoys.length ? syncedDecoys : (profile.decoy_domains || []),
    active_decoy_domain: syncedDecoys[0] || profile.active_decoy_domain || '',
    intensity: val('intensity'),
    session_minutes: parseInt(val('session-min')),
  };
  try {
    await window.go.main.App.SaveProfile(p);
    profile = p;
    renderTagList('decoy-list', p.decoy_domains || []);
    populateHotSwap();
    addLog('ai-log', { level: 'INFO', time: now(), message: t('cover_sites_saved') });
    await refreshServersAndClients();
  } catch (e) {
    alert(e);
  }
}

function addSite() {
  const list = document.getElementById('sites-list');
  list.appendChild(siteRow(''));
}

function getSiteList() {
  return [...document.querySelectorAll('#sites-list .si')].map(i => i.value.trim()).filter(Boolean);
}

function renderSites(sites) {
  const list = document.getElementById('sites-list');
  list.innerHTML = '';
  (sites || []).forEach(s => list.appendChild(siteRow(s)));
}

function siteRow(val_) {
  const row = document.createElement('div');
  row.className = 'site-row';
  const inp = document.createElement('input');
  inp.className = 'si';
  inp.type = 'text';
  inp.value = val_;
  inp.placeholder = 'example.com/path';
  const del = document.createElement('button');
  del.className = 'btn-sm danger';
  del.textContent = 'x';
  del.onclick = () => row.remove();
  row.appendChild(inp);
  row.appendChild(del);
  return row;
}

function normalizeCoverListsFromProfile(p) {
  const incoming = Array.isArray(p?.cover_lists) ? p.cover_lists : [];
  if (incoming.length > 0) {
    return incoming.map((l, i) => ({
      id: l.id || `list-${i + 1}`,
      name: (l.name || '').trim() || `List ${i + 1}`,
      sites: Array.isArray(l.sites) ? [...l.sites] : [],
    }));
  }
  const legacySites = Array.isArray(p?.cover_sites) ? p.cover_sites : [];
  const legacyName = (p?.behavior_profile || '').trim() || 'Custom';
  return [{ id: 'legacy', name: legacyName, sites: [...legacySites] }];
}

function extractDomainsFromSites(sites) {
  const out = [];
  (sites || []).forEach(raw => {
    const s = String(raw || '').trim();
    if (!s) return;
    let host = '';
    try {
      const u = s.includes('://') ? new URL(s) : new URL('https://' + s);
      host = (u.hostname || '').toLowerCase();
    } catch (_) {
      host = s.split('/')[0].toLowerCase();
    }
    host = host.replace(/^www\./, '');
    if (host && !out.includes(host)) out.push(host);
  });
  return out;
}

function syncDecoyDomainsFromActiveCoverList() {
  if (!profile) return;
  syncActiveCoverListSitesFromEditor();
  const active = getActiveCoverList();
  const domains = extractDomainsFromSites(active?.sites || []);
  if (!domains.length) return;
  profile.decoy_domains = domains;
  profile.active_decoy_domain = domains[0];
  renderTagList('decoy-list', domains);
  populateHotSwap();
}

function getActiveCoverList() {
  if (!coverLists.length) return null;
  const idx = coverLists.findIndex(l => l.id === activeCoverListId);
  return coverLists[idx >= 0 ? idx : 0];
}

function renderCoverListSelector() {
  const sel = document.getElementById('cover-list-select');
  if (!sel) return;
  sel.innerHTML = '';
  coverLists.forEach(l => {
    const opt = document.createElement('option');
    opt.value = l.id;
    opt.textContent = l.name;
    sel.appendChild(opt);
  });
  if (activeCoverListId) sel.value = activeCoverListId;
}

function syncActiveCoverListSitesFromEditor() {
  const active = getActiveCoverList();
  if (!active) return;
  active.sites = getSiteList();
}

function loadActiveCoverListIntoEditor() {
  const active = getActiveCoverList();
  if (!active) {
    renderSites([]);
    setVal('cover-list-name', '');
    return;
  }
  renderSites(active.sites || []);
  setVal('cover-list-name', active.name || '');
}

function onCoverListChange(id) {
  syncActiveCoverListSitesFromEditor();
  activeCoverListId = id;
  loadActiveCoverListIntoEditor();
  syncDecoyDomainsFromActiveCoverList();
}

function createCoverList() {
  const name = (val('cover-list-name') || '').trim() || `List ${coverLists.length + 1}`;
  const id = `list-${Date.now()}`;
  syncActiveCoverListSitesFromEditor();
  coverLists.push({ id, name, sites: [] });
  activeCoverListId = id;
  renderCoverListSelector();
  loadActiveCoverListIntoEditor();
  syncDecoyDomainsFromActiveCoverList();
}

function renameCoverList() {
  const active = getActiveCoverList();
  if (!active) return;
  const name = (val('cover-list-name') || '').trim();
  if (!name) {
    alert(t('list_name_empty'));
    return;
  }
  active.name = name;
  renderCoverListSelector();
  loadActiveCoverListIntoEditor();
  syncDecoyDomainsFromActiveCoverList();
}

function deleteCoverList() {
  if (coverLists.length <= 1) {
    alert(t('list_required'));
    return;
  }
  const active = getActiveCoverList();
  if (!active) return;
  if (!confirm(t('delete_list_confirm', { name: active.name }))) return;
  coverLists = coverLists.filter(l => l.id !== active.id);
  activeCoverListId = coverLists[0].id;
  renderCoverListSelector();
  loadActiveCoverListIntoEditor();
  syncDecoyDomainsFromActiveCoverList();
}

async function refreshMemory() {
  try {
    const st = await window.go.main.App.GetMemoryStats();
    document.getElementById('mem-stats').innerHTML = `
      <div class="stat"><div class="stat-v">${st.total}</div><div class="stat-l">${t('records')}</div></div>
      <div class="stat"><div class="stat-v" style="color:var(--ok)">${st.successes}</div><div class="stat-l">${t('success')}</div></div>
      <div class="stat"><div class="stat-v" style="color:var(--err)">${st.failures}</div><div class="stat-l">${t('failure')}</div></div>
      <div class="stat"><div class="stat-v" style="color:var(--warn)">${st.timeouts || 0}</div><div class="stat-l">${t('timeout')}</div></div>
      <div class="stat"><div class="stat-v">${(st.avg_score || 0).toFixed(2)}</div><div class="stat-l">${t('avg_score')}</div></div>
      <div class="stat"><div class="stat-v">${st.oldest_days}</div><div class="stat-l">${t('days')}</div></div>`;

    const entries = await window.go.main.App.GetMemoryEntries(50);
    const el = document.getElementById('mem-entries');
    el.innerHTML = '';
    (entries || []).forEach(e => {
      const sc = e.score > 0.65 ? 'sc-ok' : e.score < 0.35 ? 'sc-bad' : 'sc-neu';
      const icon = e.outcome === 'success' ? 'OK' : e.outcome === 'failure' ? 'X' : '.';
      const urls = (e.action?.cover_urls || []).join(', ');
      const r = document.createElement('div');
      r.className = 'mem-row';
      r.innerHTML = `<span class="mem-sc ${sc}">${icon} ${e.score.toFixed(2)}</span>
        <span>${e.context?.day_type} ${e.context?.hour}:00 | ${e.context?.rtt_trend}</span>
        <span class="mem-note">${esc(urls)}</span>
        <span style="color:var(--muted);font-size:10px">${esc(e.note || '')}</span>`;
      el.appendChild(r);
    });
  } catch (_) {}
}

async function exportMem() {
  try { await window.go.main.App.ExportMemory(); } catch (e) { alert(e); }
}

async function importMem() {
  try { await window.go.main.App.ImportMemory(); await refreshMemory(); } catch (e) { alert(e); }
}

async function clearMem() {
  if (!confirm(t('clear_memory_confirm'))) return;
  await window.go.main.App.ClearMemory();
  await refreshMemory();
}

async function runDiag() {
  try {
    const r = await window.go.main.App.RunDiagnostics();
    renderDiag('diag-local', r.local);
    renderDiag('diag-server', r.server);
  } catch (e) {
    console.error(e);
  }
}

function renderDiag(id, items) {
  const el = document.getElementById(id);
  el.innerHTML = '';
  (items || []).forEach(item => {
    const row = document.createElement('div');
    row.className = 'diag-row';
    row.innerHTML = `<span class="di">${item.ok ? 'OK' : 'X'}</span>
      <span class="dn">${esc(item.name)}</span>
      <span class="dd">${esc(item.detail)}</span>`;
    el.appendChild(row);
  });
}

async function refreshServersAndClients() {
  const serverSel = document.getElementById('s-server-select');
  const clientSel = document.getElementById('s-client-select');
  if (!serverSel || !clientSel) return;
  serverSel.innerHTML = '';
  clientSel.innerHTML = '';
  try {
    const servers = await window.go.main.App.ListServers();
    const activeServerId = await window.go.main.App.GetActiveServerID();
    (servers || []).forEach(s => {
      const opt = document.createElement('option');
      opt.value = s.id;
      const host = s.vps_host ? ` (${s.vps_host})` : '';
      opt.textContent = `${s.name}${host}`;
      serverSel.appendChild(opt);
    });
    if (activeServerId) serverSel.value = activeServerId;

    const clients = await window.go.main.App.ListClients();
    const activeClientId = await window.go.main.App.GetActiveClientID();
    (clients || []).forEach(c => {
      const opt = document.createElement('option');
      opt.value = c.id;
      opt.textContent = c.name;
      clientSel.appendChild(opt);
    });
    if (activeClientId) clientSel.value = activeClientId;
  } catch (e) {
    console.warn('servers/clients:', e);
  }
}

async function selectServer(id) {
  if (!id) return;
  try {
    await window.go.main.App.SelectServer(id);
    await loadProfile();
    await refreshServersAndClients();
    await refreshStatus();
  } catch (e) {
    alert(t('select_profile_error') + ': ' + e);
  }
}

async function createServer() {
  const name = val('s-server-name').trim() || `Server ${new Date().toLocaleString()}`;
  try {
    await window.go.main.App.CreateServer(name);
    setVal('s-server-name', '');
    await loadProfile();
    await refreshServersAndClients();
    await refreshStatus();
  } catch (e) {
    alert(t('create_profile_error') + ': ' + e);
  }
}

async function deleteActiveServer() {
  const id = document.getElementById('s-server-select')?.value;
  if (!id) return;
  if (!confirm(t('delete_profile_confirm'))) return;
  try {
    await window.go.main.App.DeleteServer(id);
    await loadProfile();
    await refreshServersAndClients();
    await refreshStatus();
  } catch (e) {
    alert(t('delete_profile_error') + ': ' + e);
  }
}

async function selectClient(id) {
  if (!id) return;
  try {
    await window.go.main.App.SelectClient(id);
    await loadProfile();
    await refreshServersAndClients();
    await refreshStatus();
  } catch (e) {
    alert(t('select_profile_error') + ': ' + e);
  }
}

async function createClient() {
  const name = val('s-client-name').trim() || `Client ${new Date().toLocaleString()}`;
  try {
    await window.go.main.App.CreateClient(name);
    setVal('s-client-name', '');
    await loadProfile();
    await refreshServersAndClients();
    await refreshStatus();
  } catch (e) {
    alert(t('create_profile_error') + ': ' + e);
  }
}

async function deleteActiveClient() {
  const id = document.getElementById('s-client-select')?.value;
  if (!id) return;
  if (!confirm(t('delete_profile_confirm'))) return;
  try {
    await window.go.main.App.DeleteClient(id);
    await loadProfile();
    await refreshServersAndClients();
    await refreshStatus();
  } catch (e) {
    alert(t('delete_profile_error') + ': ' + e);
  }
}

async function exportActiveProfile() {
  try {
    await window.go.main.App.ExportActiveProfile();
  } catch (e) {
    alert(t('export_profile_error') + ': ' + e);
  }
}

async function importProfile() {
  try {
    await window.go.main.App.ImportProfile();
    await loadProfile();
    await refreshServersAndClients();
    await refreshStatus();
  } catch (e) {
    alert(t('import_profile_error') + ': ' + e);
  }
}

async function loadProfile() {
  try {
    profile = await window.go.main.App.GetProfile();
    setVal('s-server-name', profile.server_name || '');
    setVal('s-client-name', profile.client_name || '');
    setVal('d-host', profile.vps_host);
    setVal('d-port', profile.vps_port || 22);
    setVal('d-user', profile.vps_user || 'root');
    renderTagList('decoy-list', profile.decoy_domains || []);
    coverLists = normalizeCoverListsFromProfile(profile);
    activeCoverListId = profile.active_cover_list_id || (coverLists[0] ? coverLists[0].id : '');
    if (!coverLists.find(l => l.id === activeCoverListId) && coverLists[0]) {
      activeCoverListId = coverLists[0].id;
    }
    renderCoverListSelector();
    loadActiveCoverListIntoEditor();
    syncDecoyDomainsFromActiveCoverList();
    setVal('intensity', profile.intensity);
    setVal('session-min', profile.session_minutes);
    setVal('ai-prompt-text', profile.ai_system_prompt || '');
    closePromptEditor();
    setVal('s-ai-model', profile.lmstudio_model || '');
    if (document.getElementById('s-adaptive-enabled'))
      document.getElementById('s-adaptive-enabled').checked = profile.adaptive_control !== false;
    setVal('s-adaptive-mode', profile.adaptive_mode || 'balanced');
    setVal('s-rag-timeout-policy', profile.memory_timeout_policy || 'base');
    setVal('s-awg-exe', profile.awg_exe_path);
    setVal('s-awg-iface', profile.awg_interface);
    setVal('s-hs-interval', profile.hotswap_interval_min);
    if (document.getElementById('s-hs-enabled'))
      document.getElementById('s-hs-enabled').checked = profile.hotswap_enabled;
    setText('ci-port', profile.awg_listen_port || '-');
    setText('ci-decoy', profile.active_decoy_domain || '-');
    populateHotSwap();

    const hint = document.getElementById('conn-hint');
    if (!profile.deployed) hint.textContent = t('run_deploy_hint');
    else hint.textContent = t('deploy_done_hint');
  } catch (e) {
    console.error('loadProfile:', e);
  }
}

async function refreshLMStudioModels() {
  const sel = document.getElementById('s-ai-model-select');
  if (!sel) return;
  sel.innerHTML = '';
  const first = document.createElement('option');
  first.value = '';
  first.textContent = t('auto_not_selected');
  sel.appendChild(first);

  try {
    const models = await window.go.main.App.GetLMStudioModels();
    (models || []).forEach(m => {
      const opt = document.createElement('option');
      opt.value = m;
      opt.textContent = m;
      sel.appendChild(opt);
    });
    if (profile?.lmstudio_model) sel.value = profile.lmstudio_model;
  } catch (e) {
    console.warn('LM Studio models:', e);
  }
}

function applySelectedLMStudioModel() {
  const sel = document.getElementById('s-ai-model-select');
  if (!sel) return;
  setVal('s-ai-model', sel.value || '');
}

async function saveSettings() {
  if (!profile) return;
  const p = {
    ...profile,
    lmstudio_model: val('s-ai-model'),
    adaptive_control: document.getElementById('s-adaptive-enabled').checked,
    adaptive_mode: val('s-adaptive-mode') || 'balanced',
    memory_timeout_policy: val('s-rag-timeout-policy') || 'base',
    awg_exe_path: val('s-awg-exe'),
    awg_interface: val('s-awg-iface'),
    hotswap_enabled: document.getElementById('s-hs-enabled').checked,
    hotswap_interval_min: parseInt(val('s-hs-interval')),
  };
  try {
    await window.go.main.App.SaveProfile(p);
    profile = p;
    alert(t('settings_saved'));
    await refreshServersAndClients();
  } catch (e) {
    alert(t('error_prefix') + ': ' + e);
  }
}

function renderTagList(id, items) {
  const list = document.getElementById(id);
  list.innerHTML = '';
  (items || []).forEach(v => list.appendChild(tagRow(id, v)));
}

function addDecoy() {
  document.getElementById('decoy-list').appendChild(tagRow('decoy-list', ''));
}

function tagRow(_listId, v) {
  const row = document.createElement('div');
  row.className = 'site-row';
  const inp = document.createElement('input');
  inp.type = 'text';
  inp.value = v;
  inp.placeholder = 'microsoft.com';
  inp.className = 'ti';
  const del = document.createElement('button');
  del.className = 'btn-sm danger';
  del.textContent = 'x';
  del.onclick = () => row.remove();
  row.appendChild(inp);
  row.appendChild(del);
  return row;
}

function getTagListValues(id) {
  return [...document.querySelectorAll('#' + id + ' .ti')].map(i => i.value.trim()).filter(Boolean);
}

function addLog(id, entry) {
  const log = document.getElementById(id);
  if (!log) return;
  const d = document.createElement('div');
  d.className = 'll ' + (entry.level || 'INFO');
  d.innerHTML = `<span class="lt">${entry.time || now()}</span>${esc(entry.message)}`;
  log.appendChild(d);
  aiLogLines.push(d);
  if (aiLogLines.length > MAX_LOG) aiLogLines.shift().remove();
  log.scrollTop = log.scrollHeight;
}

function clearAILog() {
  document.getElementById('ai-log').innerHTML = '';
}

function open_(url) { window.runtime.BrowserOpenURL(url); }
function val(id) { return document.getElementById(id)?.value || ''; }
function setVal(id, v) { const e = document.getElementById(id); if (e && v !== undefined && v !== null) e.value = v; }
function setText(id, v) { const e = document.getElementById(id); if (e) e.textContent = v; }
function now() { return new Date().toTimeString().slice(0, 8); }
function esc(s) { return String(s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
