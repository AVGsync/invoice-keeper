#!/usr/bin/env python3
"""Сид демо-данных через API (только stdlib).

Создаёт персоны, сотрудников, регулярные платежи (RUB/USD/EUR, monthly/weekly)
и отмечает часть прошлых периодов оплаченными. Периоды генерирует сам backend.
Повторный запуск безопасен: существующие персоны, логины и платежи (по описанию)
пропускаются.

    python3 scripts/seed.py                       # http://localhost:8080, admin/admin
    API_URL=https://<домен>/api/v1 ADMIN_LOGIN=admin ADMIN_PASSWORD=... python3 scripts/seed.py

Пароль всех созданных сотрудников — SEED_PASSWORD (по умолчанию Demo12345).
"""

from __future__ import annotations

import json
import os
import ssl
import sys
import urllib.error
import urllib.request
from datetime import date, timedelta
from http.cookies import SimpleCookie

API = os.environ.get("API_URL", "http://localhost:8080/api/v1").rstrip("/")
ADMIN_LOGIN = os.environ.get("ADMIN_LOGIN", "admin")
ADMIN_PASSWORD = os.environ.get("ADMIN_PASSWORD", "admin")
SEED_PASSWORD = os.environ.get("SEED_PASSWORD", "Demo12345")
# Самоподписанный сертификат на сервере: SEED_INSECURE=1.
CTX = ssl._create_unverified_context() if os.environ.get("SEED_INSECURE") else None

PERSONAS = [
    ("Бухгалтер", "Оплачивает аренду, налоги и счета"),
    ("DevOps", "Хостинг, домены, облака"),
    ("Офис-менеджер", "Офис, связь, хозяйственные расходы"),
    ("Маркетолог", "Рекламные кабинеты и сервисы"),
]

# login, ФИО, персона
USERS = [
    ("olga", "Ольга Иванова", "Бухгалтер"),
    ("ivan", "Иван Петров", "DevOps"),
    ("maria", "Мария Смирнова", "Офис-менеджер"),
    ("petr", "Пётр Сидоров", "Маркетолог"),
]

TODAY = date.today()


def months_ago(n: int, day: int) -> str:
    y, m = TODAY.year, TODAY.month - n
    while m <= 0:
        m += 12
        y -= 1
    return date(y, m, min(day, 28)).isoformat()


# описание, сумма (в основных единицах), валюта, частота, интервал, старт, ответственный
PAYMENTS = [
    ("Аренда офиса", 150000, "RUB", "monthly", 1, months_ago(4, 5), "olga"),
    ("Бухгалтерия: 1С облако", 4900, "RUB", "monthly", 1, months_ago(3, 12), "olga"),
    ("Интернет и телефония", 3500, "RUB", "monthly", 1, months_ago(3, 20), "maria"),
    ("Уборка офиса", 6000, "RUB", "weekly", 2, (TODAY - timedelta(days=40)).isoformat(), "maria"),
    ("Вода и кофе", 2800, "RUB", "monthly", 1, months_ago(2, 25), "maria"),
    ("Хостинг Hetzner", 89.9, "EUR", "monthly", 1, months_ago(4, 1), "ivan"),
    ("Домены и DNS", 15, "USD", "monthly", 1, months_ago(2, 8), "ivan"),
    ("GitHub Team", 48, "USD", "monthly", 1, months_ago(3, 15), "ivan"),
    ("Sentry", 26, "USD", "monthly", 1, months_ago(1, 3), "ivan"),
    ("Figma Professional", 45, "USD", "monthly", 1, months_ago(3, 18), "petr"),
    ("Notion Plus", 40, "EUR", "monthly", 1, months_ago(2, 22), "petr"),
    ("Яндекс Директ", 30000, "RUB", "monthly", 1, months_ago(2, 10), "petr"),
]

# Сколько последних прошедших периодов каждого платежа оставить неоплаченными
# (будут просрочены). У остальных платежей все прошлые периоды оплачены.
LEAVE_UNPAID = {"Яндекс Директ": 1, "Домены и DNS": 1, "Вода и кофе": 1}
# Эти платежи оплачивались с опозданием на 3 дня.
PAID_LATE = {"Интернет и телефония"}

token = ""


def call(method: str, path: str, body: object | None = None) -> tuple[int, object, object]:
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(API + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, context=CTX, timeout=30) as resp:
            raw = resp.read().decode()
            return resp.status, (json.loads(raw) if raw.strip().startswith(("{", "[")) else raw), resp.headers
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode(), e.headers


def must(status: int, payload: object, what: str, ok: tuple[int, ...] = (200, 201, 204)) -> None:
    if status not in ok:
        sys.exit(f"{what}: HTTP {status}: {payload}")


def login() -> None:
    global token
    status, payload, headers = call("POST", "/auth/login", {"login": ADMIN_LOGIN, "password": ADMIN_PASSWORD})
    must(status, payload, "вход админа")
    for header in headers.get_all("Set-Cookie") or []:
        c = SimpleCookie()
        c.load(header)
        if "AccessToken" in c:
            token = c["AccessToken"].value
    if not token:
        sys.exit("вход админа: в ответе нет куки AccessToken")


def seed_personas() -> dict[str, str]:
    status, payload, _ = call("GET", "/personas")
    must(status, payload, "список персон")
    existing = {p["name"].lower(): p["id"] for p in payload["items"]}
    for name, descr in PERSONAS:
        if name.lower() in existing:
            continue
        status, payload, _ = call("POST", "/personas", {"name": name, "description": descr})
        must(status, payload, f"персона {name}")
        existing[name.lower()] = payload["id"]
        print(f"+ персона {name}")
    return existing


def user_ids() -> dict[str, str]:
    status, payload, _ = call("GET", "/users")
    must(status, payload, "список пользователей")
    return {u["login"]: u["id"] for u in payload["items"]}


def seed_users(personas: dict[str, str]) -> dict[str, str]:
    ids = user_ids()
    for login_, full_name, persona in USERS:
        if login_ in ids:
            continue
        body = {"login": login_, "full_name": full_name, "password": SEED_PASSWORD,
                "persona_id": personas[persona.lower()]}
        status, payload, _ = call("POST", "/users", body)
        must(status, payload, f"пользователь {login_}")
        print(f"+ сотрудник {full_name} ({login_} / {SEED_PASSWORD})")
    # POST /users отвечает 201 без тела — id берём из списка.
    return user_ids()


def seed_payments(users: dict[str, str]) -> None:
    status, payload, _ = call("GET", "/recurring-payments")
    must(status, payload, "список регулярных платежей")
    items = payload if isinstance(payload, list) else payload.get("items", [])
    existing = {p["description"] for p in items}
    for descr, amount, cur, freq, interval, start, who in PAYMENTS:
        if descr in existing:
            continue
        body = {"description": descr, "amount": round(amount * 100), "currency": cur,
                "frequency": freq, "interval": interval, "start_date": start,
                "assignee_id": users[who]}
        status, payload, _ = call("POST", "/recurring-payments", body)
        must(status, payload, f"платёж {descr}")
        print(f"+ платёж {descr}: {amount} {cur}, {freq}/{interval}, с {start}")


def mark_paid() -> None:
    status, payload, _ = call("GET", "/payment-occurrences")
    must(status, payload, "список периодов")
    by_payment: dict[str, list[dict]] = {}
    for occ in payload["items"]:
        if occ.get("paid_date") or occ["due_date"] >= TODAY.isoformat():
            continue
        by_payment.setdefault(occ["description"], []).append(occ)
    paid = 0
    for descr, occs in by_payment.items():
        occs.sort(key=lambda o: o["due_date"])
        keep = LEAVE_UNPAID.get(descr, 0)
        for occ in occs[: len(occs) - keep] if keep else occs:
            due = date.fromisoformat(occ["due_date"])
            pay_day = min(due + timedelta(days=3), TODAY) if descr in PAID_LATE else due
            status, body, _ = call("PATCH", f"/payment-occurrences/{occ['id']}",
                                   {"action": "confirm", "paid_date": pay_day.isoformat()})
            must(status, body, f"оплата {descr} {occ['due_date']}")
            paid += 1
    print(f"отмечено оплаченными: {paid}")


def main() -> None:
    print(f"API: {API}")
    login()
    personas = seed_personas()
    users = seed_users(personas)
    seed_payments(users)
    mark_paid()
    print("готово")


if __name__ == "__main__":
    main()
