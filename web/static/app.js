// Общий клиент API и работа с токеном.
//
// Токен лежит в localStorage. Это уязвимо к XSS — любой скрипт на странице
// его прочитает. Надёжнее httpOnly-кука, но она требует правок на сервере
// (установка куки и защита от CSRF), поэтому пока так.

const TOKEN_KEY = "tinyurl_token";
const USER_KEY = "tinyurl_user";

const auth = {
    token: () => localStorage.getItem(TOKEN_KEY),

    user() {
        const raw = localStorage.getItem(USER_KEY);
        return raw ? JSON.parse(raw) : null;
    },

    save(token, user) {
        localStorage.setItem(TOKEN_KEY, token);
        localStorage.setItem(USER_KEY, JSON.stringify(user));
    },

    clear() {
        localStorage.removeItem(TOKEN_KEY);
        localStorage.removeItem(USER_KEY);
    },

    isLoggedIn: () => Boolean(localStorage.getItem(TOKEN_KEY)),
};

// ApiError несёт HTTP-статус, чтобы вызывающий мог отличить 401 от 400.
class ApiError extends Error {
    constructor(status, message) {
        super(message);
        this.status = status;
    }
}

async function api(method, path, body) {
    const headers = {};
    if (body !== undefined) {
        headers["Content-Type"] = "application/json";
    }

    const token = auth.token();
    if (token) {
        headers["Authorization"] = "Bearer " + token;
    }

    const response = await fetch(path, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
    });

    // Токен протух или подделан — выкидываем на вход.
    if (response.status === 401 && auth.isLoggedIn()) {
        auth.clear();
        location.href = "/login.html";
        throw new ApiError(401, "Сессия истекла");
    }

    // 204 No Content — тела нет, парсить нечего.
    if (response.status === 204) {
        return null;
    }

    const data = await response.json().catch(() => null);

    if (!response.ok) {
        throw new ApiError(response.status, translateError(data?.error, response.status));
    }

    return data;
}

// Бэкенд отвечает по-английски; здесь сопоставляем понятным текстом.
// Точное совпадение, а не подстрока: тексты ошибок заданы в коде сервиса.
const ERRORS = {
    "invalid username or password": "Неверный логин или пароль",
    "user with this name or email already exists": "Пользователь с таким именем или почтой уже есть",
    "username is required": "Введите имя",
    "username must be at least 3 characters": "Имя короче трёх символов",
    "username must be at most 32 characters": "Имя длиннее 32 символов",
    "email is invalid": "Некорректная почта",
    "password must be at least 8 characters": "Пароль короче восьми символов",
    "password must be at most 72 bytes": "Слишком длинный пароль",
    "invalid URL": "Не похоже на ссылку",
    "not http(s) scheme": "Ссылка должна начинаться с http:// или https://",
    "cycle href": "Нельзя сократить ссылку на сам сервис",
    "URL is too long": "Слишком длинная ссылка",
    "slot not found": "Слот не найден",
    "request body must be a valid JSON object": "Некорректный запрос",
};

function translateError(message, status) {
    if (message && ERRORS[message]) {
        return ERRORS[message];
    }
    if (message) {
        return message;
    }
    return `Ошибка сервера (${status})`;
}

// ── Утилиты страниц ────────────────────────────────────────────────

// Показывает сообщение в элементе #message.
function showMessage(text, kind = "error") {
    const box = document.getElementById("message");
    if (!box) return;

    box.textContent = text;
    box.className = "message " + kind;
}

function clearMessage() {
    const box = document.getElementById("message");
    if (box) {
        box.textContent = "";
        box.className = "message";
    }
}

// Отправляет форму, блокируя кнопку на время запроса, чтобы двойной клик
// не создал два запроса.
async function submitForm(button, handler) {
    clearMessage();
    button.disabled = true;

    try {
        await handler();
    } catch (err) {
        showMessage(err.message);
    } finally {
        button.disabled = false;
    }
}

// Рисует ссылки в шапке в зависимости от того, вошёл ли пользователь.
function renderNav() {
    const nav = document.getElementById("nav-links");
    if (!nav) return;

    if (auth.isLoggedIn()) {
        const user = auth.user();
        nav.innerHTML = `
            <a href="/dashboard.html">Мои ссылки</a>
            <a href="#" id="logout">Выйти${user ? " (" + escapeHtml(user.name) + ")" : ""}</a>`;

        document.getElementById("logout").addEventListener("click", (e) => {
            e.preventDefault();
            auth.clear();
            location.href = "/";
        });
    } else {
        nav.innerHTML = `
            <a href="/login.html">Вход</a>
            <a href="/register.html">Регистрация</a>`;
    }
}

// Экранирование: имя пользователя и адреса ссылок подставляются в разметку,
// а туда может попасть что угодно.
function escapeHtml(value) {
    return String(value).replace(/[&<>"']/g, (ch) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
    })[ch]);
}

// Уводит на вход, если страница требует авторизации.
function requireAuth() {
    if (!auth.isLoggedIn()) {
        location.href = "/login.html";
        return false;
    }
    return true;
}

document.addEventListener("DOMContentLoaded", renderNav);
