// wb-extract-cookies.js — скрипт для извлечения сессионных данных WB Stream.
//
// Как использовать:
// 1. Откройте WB Stream (сайт https://stream.wb.ru/ или Electron-приложение)
// 2. Войдите в свой аккаунт
// 3. Откройте DevTools (F12)
// 4. Вставьте этот скрипт в консоль и нажмите Enter
// 5. Скопируйте полученный JSON — его можно вставить в поле «JSON dump»
//    в настройках WB Account на панели AudioBot
//
// Для Electron-приложения WB Stream:
//   - Запустите: "C:\Program Files\WB Stream\WB Stream.exe" --remote-debugging-port=9222
//   - В Chrome откройте chrome://inspect/#devices
//   - Нажмите «inspect fallback», откройте Console, вставьте скрипт

(function() {
    console.log("%c--- ЗАПУСК СБОРА ДАННЫХ СЕССИИ WB STREAM ---", "color: cyan; font-weight: bold;");

    const sessionData = {
        timestamp: new Date().toISOString(),
        userAgent: navigator.userAgent,
        url: window.location.href,
        cookies: document.cookie,
        localStorage: {},
        sessionStorage: {},
    };

    // Сбор LocalStorage
    for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        sessionData.localStorage[key] = localStorage.getItem(key);
    }

    // Сбор SessionStorage
    for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        sessionData.sessionStorage[key] = sessionStorage.getItem(key);
    }

    console.log("%c--- СБОР ЗАВЕРШЁН. СКОПИРУЙТЕ JSON НИЖЕ ---", "color: green; font-weight: bold;");
    console.log(JSON.stringify(sessionData, null, 2));
})();
