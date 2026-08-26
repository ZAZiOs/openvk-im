# Документация по endpointам
## Авторизация и получение ключа сессии

Для доступа к защищенным эндпоинтам необходимо передать уникальный ключ сессии (`key`). Этот ключ должен быть предварительно сгенерирован на стороне бэкенда и сохранен в хранилище Redis.

### 1. Генерация и сохранение ключа
1.  Создайте криптографически стойкую случайную строку (например, UUID v4 или случайный хеш).
2.  Запишите пару "ключ-значение" в Redis, где:
    * **Key**: `"im:session:api:"` + Сгенерированная строка.
    * **Value**: ID пользователя (`userID`).
    * **TTL**: Установите время жизни сессии (например, 24 часа).
---

### 2. Способы передачи ключа в запросах
Middleware поддерживает два способа передачи ключа. Можно выбрать наиболее удобный для своего стека:

#### Способ А: Через HTTP-заголовок (Рекомендуется)
Используйте стандартный заголовок `Authorization` с типом `Bearer`.
* **Заголовок:** `Authorization: Bearer <ключ>`

#### Способ Б: Через Query-параметр
Ключ передается прямо в URL.
* **URL:** `https://im.openvk.org/endpoint?key=<ключ>`

---

### 3. Обработка ошибок авторизации
Если ключ не передан или сессия в Redis истекла, API вернет ошибку в формате JSON с кодом **5** (внутренний код бизнес-логики):

| HTTP Код | JSON Code | Сообщение | Причина |
| :--- | :--- | :--- | :--- |
| **200/401** | `5` | `User authorization failed: no key passed` | Ключ отсутствует в Header и в Query. |
| **200/401** | `5` | `User authorization failed: invalid session` | Ключ передан, но не найден в Redis (истек или удален). |

---

### 4. Пример для PHP (cURL)
```php
$sessionKey = "your_generated_redis_key";
$ch = curl_init("https://api.example.com/v1/user/profile");

curl_setopt($ch, CURLOPT_HTTPHEADER, [
    'Authorization: Bearer ' . $sessionKey,
    'Content-Type: application/json'
]);
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);

$response = curl_exec($ch);
$data = json_decode($response, true);

if (isset($data['code']) && $data['code'] === 5) {
    // Редирект на логин или обновление сессии
    die("Ошибка авторизации: " . $data['message']);
}
```
## Описание методов

Базовый URL для вызова методов: https://im.openvk.org/method/messages.methodName?key={access}
Каждый эндпоинт требует ключ авторизации.

### im.getUnreadMessages
Возвращает:
```json
"response": { 
    "messages": uint
    }
```

### im.getUnreadConversations
Возвращает:
```json
"response": { 
    "count": uint
    }
```

### im.checkPeerExist
Проверяет наличие переписки / диалога с указанным `peer_id` (или `user_id`, `chat_id`).

Параметры:
* `peer_id` (int64) — ID диалога/беседы (ID пользователя, ID беседы `2000000000 + id` или ID сообщества `-id`).

Возвращает:
```json
"response": { 
    "exists": bool
    }
```

