# VK API 5.20 Documentation (Messages)

> Documentation for the VK API Messages section around version 5.20 (August 2014).

---

## Table of Contents

- [Objects](#objects)
  - [Message Object](#message-object)
  - [Chat Object](#chat-object)
- [Methods](#methods)
  - [messages.get](#messagesget)
  - [messages.getDialogs](#messagesgetdialogs)
  - [messages.getById](#messagesgetbyid)
  - [messages.search](#messagessearch)
  - [messages.getHistory](#messagesgethistory)
  - [messages.send](#messagessend)
  - [messages.delete](#messagesdelete)
  - [messages.deleteDialog](#messagesdeletedialog)
  - [messages.restore](#messagesrestore)
  - [messages.markAsRead](#messagesmarkasread)
  - [messages.markAsImportant](#messagesmarkasimportant)
  - [messages.getLongPollServer](#messagesgetlongpollserver)
  - [messages.getLongPollHistory](#messagesgetlongpollhistory)
  - [messages.getChat](#messagesgetchat)
  - [messages.createChat](#messagescreatechat)
  - [messages.editChat](#messageseditchat)
  - [messages.getChatUsers](#messagesgetchatusers)
  - [messages.setActivity](#messagessetactivity)
  - [messages.searchDialogs](#messagessearchdialogs)
  - [messages.addChatUser](#messagesaddchatuser)
  - [messages.removeChatUser](#messagesremovechatuser)
  - [messages.getLastActivity](#messagesgetlastactivity)
  - [messages.setChatPhoto](#messagessetchatphoto)
  - [messages.deleteChatPhoto](#messagesdeletechatphoto)
- [User Long Poll API](#user-long-poll-api-vk-api-520--protocol-v0-v1)

---

## Objects

### Message Object

Describes a private message. Contains the following fields:

| Field | Type | Description |
|---|---|---|
| `id` | `positive number` | Message ID. *(Not returned for forwarded messages)* |
| `user_id` | `positive number` | For an incoming message — author user ID. For an outgoing message — receiver user ID. |
| `date` | `positive number` | Date (in Unix time) when the message was sent. |
| `read_state` | `flag (0 or 1)` | Message status (`0` — not read, `1` — read). *(Not returned for forwarded messages)* |
| `out` | `flag (0 or 1)` | Message type (`0` — received, `1` — sent). *(Not returned for forwarded messages)* |
| `title` | `string` | Title of message or chat. |
| `body` | `string` | Body text of the message. |
| `attachments` | `array` | Array of media attachments. |
| `fwd_messages` | `array` | Array of forwarded messages (if any). |
| `emoji` | `flag (0 or 1)` | Whether the message contains smiles (`0` — no, `1` — yes). |
| `deleted` | `flag (0 or 1)` | Whether the message is deleted (`0` — no, `1` — yes). |

#### Additional fields for group chats only

| Field | Type | Description |
|---|---|---|
| `chat_id` | `positive number` | Chat ID. |
| `chat_active` | `list of numbers` | User IDs of chat participants (comma-separated positive numbers). |
| `users_count` | `positive number` | Number of chat participants. |
| `admin_id` | `positive number` | ID of the user who started the chat. |
| `photo_50` | `string` | URL of chat image with width size of 50px. |
| `photo_100` | `string` | URL of chat image with width size of 100px. |
| `photo_200` | `string` | URL of chat image with width size of 200px. |

---

### Chat Object

Describes a multi-user chat. Contains the following fields:

| Field | Type | Description |
|---|---|---|
| `id` | `positive number` | Chat ID. |
| `type` | `string` | Type of chat (`chat`). |
| `title` | `string` | Chat title. |
| `admin_id` | `positive number` | ID of the chat creator / starter. |
| `users` | `array of numbers` | List of chat participants' IDs. |

---

## Methods

### messages.get

Returns a list of the current user's incoming or outgoing private messages.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `out` | `int` | `1` — return outgoing messages, `0` — return incoming messages *(default)*. |
| `offset` | `positive number` | Offset needed to return a specific subset of messages. |
| `count` | `positive number` | Number of messages to return *(default `20`, max `200`)*. |
| `time_offset` | `int` | Maximum time since a message was sent, in seconds. To return messages without time limitation, set to `0`. |
| `filters` | `int` | Filter to apply:<br>`1` — unread only<br>`2` — not from chat<br>`4` — messages from friends.<br>*If `4` is set, `1` and `2` are ignored.* |
| `preview_length` | `positive number` | Number of characters after which to truncate a previewed message (`0` for full message). Messages are truncated by words. |
| `last_message_id` | `positive number` | ID of the message received before the message that will be returned last. |

#### Result

Returns a list of **Message** objects:
```json
{
  "response": {
    "count": 100,
    "items": [ /* Message objects */ ]
  }
}
```

---

### messages.getDialogs

Returns a list of the current user's conversations (last message of each dialog).

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `offset` | `positive number` | Offset needed to return a specific subset of messages. |
| `count` | `positive number` | Number of messages to return *(default `20`, max `200`)*. |
| `preview_length` | `positive number` | Number of characters after which to truncate a previewed message (`0` for full message). Messages are truncated by words. |
| `unread` | `flag (0 or 1)` | Filter unread conversations *(accessible from version 5.14)*. |

#### Result

Returns a list of **Message** objects representing the last message of each conversation.

---

### messages.getById

Returns messages by their IDs.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `message_ids` | `list of numbers` | Message IDs separated by commas. |
| `preview_length` | `positive number` | Number of characters after which to truncate a previewed message (`0` for full message). |

#### Result

Returns a list of **Message** objects.

---

### messages.search

Returns a list of the current user's private messages that match search criteria.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `q` | `string` | **Required.** Search query string. |
| `preview_length` | `positive number` | Number of characters after which to truncate a previewed message (`0` for full message). |
| `offset` | `positive number` | Offset needed to return a specific subset of messages. |
| `count` | `positive number` | Number of messages to return *(default `20`, max `100`)*. |

#### Result

Returns a list of **Message** objects.

---

### messages.getHistory

Returns message history for the specified user or group chat.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `user_id` | `string` | ID of the user whose message history you want to return *(required if `chat_id` is not set)*. |
| `chat_id` | `positive number` | ID of the chat whose message history you want to return *(required if `user_id` is not set)*. |
| `offset` | `int` | Offset needed to return a specific subset of messages. |
| `count` | `positive number` | Number of messages to return *(default `20`, max `200`)*. |
| `start_message_id` | `int` | Starting message ID from which to return history. |
| `rev` | `int` | Sort order:<br>`1` — chronological order<br>`0` — reverse chronological order. |

#### Result

Returns a list of **Message** objects.

---

### messages.send

Sends a message.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `user_id` | `int` | User ID *(by default — current user)*. |
| `domain` | `string` | User's short address (e.g. `illarionov`). |
| `chat_id` | `positive number` | ID of conversation the message will relate to. |
| `user_ids` | `list of numbers` | IDs of message recipients (if starting a new conversation). |
| `message` | `string` | Text of the message *(required if `attachment` is not set)*. |
| `guid` | `int` | Unique ID used to prevent re-sending of the same message. |
| `lat` | `fraction` | Latitude of a check-in (-90 to 90). |
| `long` | `fraction` | Longitude of a check-in (-180 to 180). |
| `attachment` | `string` | Media attachments separated by commas (`<type><owner_id>_<media_id>`, e.g. `photo100172_166443618`). Types: `photo`, `video`, `audio`, `doc`, `wall`. |
| `forward_messages` | `string` | IDs of forwarded messages separated by commas (e.g. `123,431,544`). |

#### Result

Returns the ID of the sent message (`mid`).

---

### messages.delete

Deletes one or more messages.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `message_ids` | `list of numbers` | Message IDs separated by commas. |

#### Result

Returns `1`.

---

### messages.deleteDialog

Deletes all private messages in a conversation.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `user_id` | `string` | User ID. |
| `offset` | `positive number` | Offset needed to return a specific subset of messages. |
| `count` | `positive number` | Number of messages to delete *(max `10000`)*. If number exceeds max, method must be called multiple times. |

#### Result

Returns `1`.

---

### messages.restore

Restores a deleted message.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `message_id` | `positive number` | **Required.** ID of a previously-deleted message to restore. |

#### Result

Returns `1`.

---

### messages.markAsRead

Marks messages as read.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `message_ids` | `list of numbers` | IDs of messages to mark as read (comma-separated). |
| `peer_id` | `string` | Destination / peer ID. |
| `start_message_id` | `positive number` | ID of the message from which to mark as read. |

#### Result

Returns `1`.

---

### messages.markAsImportant

Marks and unmarks messages as important (starred).

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `message_ids` | `list of numbers` | IDs of messages to mark as important. |
| `important` | `positive number` | `1` — add star (mark important), `0` — remove star. |

#### Result

Returns a list of IDs of successfully marked messages.

---

### messages.getLongPollServer

Returns data required for connection to a Long Poll server.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `use_ssl` | `flag (0 or 1)` | `1` — use SSL. |
| `need_pts` | `flag (0 or 1)` | `1` — return `pts` field (needed for `messages.getLongPollHistory`). |

#### Result

Returns an object with `key`, `server`, and `ts` fields:
```json
{
  "response": {
    "key": "4a3d...",
    "server": "im.vk.com/lp...",
    "ts": 123456789,
    "pts": 12345
  }
}
```

---

### messages.getLongPollHistory

Returns updates in user's private messages.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `ts` | `positive number` | Last value of the `ts` parameter. |
| `pts` | `positive number` | Value of `pts` parameter. |
| `preview_length` | `positive number` | Number of characters after which to truncate a previewed message (`0` for full message). |
| `onlines` | `flag (0 or 1)` | `1` — return online status updates. |
| `events_limit` | `positive number` | Max number of events *(default `1000`, min `1000`)*. |
| `msgs_limit` | `positive number` | Max number of messages *(default `200`, min `200`)*. |
| `max_msg_id` | `positive number` | Maximum ID of the message among existing ones in the local copy. |

#### Result

Returns an object containing:
- `history` — Array of update events from Long Poll server.
- `messages` — Array of private message objects found among events with code 4.

---

### messages.getChat

Returns information about a chat.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `chat_id` | `int` | Chat ID. |
| `chat_ids` | `list of numbers` | Chat IDs separated by commas. |
| `fields` | `list of strings` | Profile fields to return for participants. |
| `name_case` | `string` | Case for declension of user names: `nom` *(default)*, `gen`, `dat`, `acc`, `ins`, `abl`. |

#### Result

Returns a list of **Chat** objects. If `fields` is set, `users` contains user objects with an extra `invited_by` field.

---

### messages.createChat

Creates a chat with several participants.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `user_ids` | `list of numbers` | **Required.** IDs of the users to be added to the chat (comma-separated positive numbers). |
| `title` | `string` | Chat title. |

#### Result

Returns the ID of the created chat (`chat_id`).

---

### messages.editChat

Edits the title of a chat.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `chat_id` | `int` | **Required.** Chat ID. |
| `title` | `string` | **Required.** New title of the chat. |

#### Result

Returns `1`.

---

### messages.getChatUsers

Returns a list of IDs of users participating in a chat.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `chat_id` | `int` | Chat ID. |
| `chat_ids` | `list of numbers` | Chat IDs separated by commas. |
| `fields` | `list of strings` | Profile fields to return. |
| `name_case` | `string` | Case for declension: `nom` *(default)*, `gen`, `dat`, `acc`, `ins`, `abl`. |

#### Result

Returns a list of IDs of chat participants (or list of user objects with `invited_by` if `fields` is set).

---

### messages.setActivity

Changes the status of a user as typing in a conversation.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `user_id` | `string` | User ID. |
| `type` | `string` | `typing` — user has started to type. |

#### Result

Returns `1`. Status is active for 10 seconds or until a message is sent.

---

### messages.searchDialogs

Returns a list of the current user's conversations that match search criteria.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `q` | `string` | Search query string. |
| `limit` | `positive number` | Number of results to return *(default `20`)*. |
| `fields` | `list of strings` | Profile fields to return. |

#### Result

Returns an object containing:
- `profiles` — Array of user objects.
- `chats` — Array of chat objects.

---

### messages.addChatUser

Adds a new user to a chat.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `chat_id` | `int` | Chat ID. |
| `user_id` | `positive number` | **Required.** ID of the user to be added to the chat. |

#### Result

Returns `1`.

#### Errors

- `103` — Out of limits.

---

### messages.removeChatUser

Allows the current user to leave a chat or removes another user from the chat.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `chat_id` | `int` | **Required.** Chat ID. |
| `user_id` | `positive number` | **Required.** ID of the user to be removed from the chat. |

#### Result

Returns `1`.

---

### messages.getLastActivity

Returns a user's current status and date of last activity.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `user_id` | `int` | **Required.** User ID. |

#### Result

Returns an object with fields:
- `online` (`0` or `1`) — User's current status (`0` — offline, `1` — online).
- `time` (`positive number`) — Date (in Unix time) of the user's last activity.

```json
{
  "response": {
    "online": 1,
    "time": 1407000000
  }
}
```

---

### messages.setChatPhoto

Sets a previously-uploaded picture as the cover picture of a chat.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `file` | `string` | **Required.** Upload URL from the `response` field returned by `photos.getChatUploadServer` upon successfully uploading an image. |

#### Result

Returns an object with the following fields:
- `message_id` — ID of the system message sent.
- `chat` — **Chat** object.

```json
{
  "response": {
    "message_id": 42,
    "chat": { /* Chat object */ }
  }
}
```

#### Errors

- `22` — Upload error.
- `1160` — Original photo was changed.

---

### messages.deleteChatPhoto

Deletes a chat's cover picture.

#### Parameters

| Parameter | Type | Description |
|---|---|---|
| `chat_id` | `positive number` | **Required.** Chat ID. |

#### Result

Returns an object with the following fields:
- `message_id` — ID of the system message sent.
- `chat` — **Chat** object.

```json
{
  "response": {
    "message_id": 43,
    "chat": { /* Chat object */ }
  }
}
```



---

## User Long Poll API (VK API 5.20 / Protocol v0-v1)

Long Polling — это технология, позволяющая получать данные о новых событиях в реальном времени с помощью «длинных запросов». Сервер удерживает соединение открытым до появления события либо до истечения времени ожидания (`wait`).

### 1. Подключение к Long Poll серверу

Перед подключением необходимо получить данные сессии (`server`, `key`, `ts`) с помощью метода `messages.getLongPollServer`.

Формат запроса к Long Poll серверу:
```http
GET http://{$server}?act=a_check&key={$key}&ts={$ts}&wait=25&mode=2
```

#### Параметры подключения:

| Параметр | Тип | Описание |
|---|---|---|
| `act` | `string` | Всегда `a_check`. |
| `key` | `string` | Секретный ключ сессии, полученный из `messages.getLongPollServer`. |
| `ts` | `integer` | Номер последнего полученного события. |
| `wait` | `integer` | Время ожидания событий в секундах (рекомендуется `25`, максимум `90`). |
| `mode` | `integer` | Флаги режима: `2` — получать вложения (`attachments`), `0` — без вложений. |

---

### 2. Формат ответа сервера

#### Пример ответа без событий (таймаут):
```json
{
  "ts": 934518069,
  "updates": []
}
```

#### Пример ответа с событиями:
```json
{
  "ts": 196851367,
  "updates": [
    [
      4,
      16929,
      1,
      85635407,
      1280307577,
      " ... ",
      "hello",
      {
        "attach1_type": "photo",
        "attach1": "123_456"
      }
    ]
  ]
}
```

#### Ошибки Long Poll:
Если срок действия ключа истёк или история потеряна, сервер возвращает код ошибки `failed`:

| Код ошибки | Описание | Действие клиента |
|---|---|---|
| `{"failed": 1, "ts": ...}` | История событий устарела. | Продолжить получение событий, используя новый `ts` из ответа. |
| `{"failed": 2}` | Истёк срок действия сессионного ключа `key`. | Запросить новые данные через `messages.getLongPollServer`. |
| `{"failed": 3}` | Информация о пользователе утрачена. | Запросить новые `key` и `ts` через `messages.getLongPollServer`. |

---

### 3. Структура событий (`updates`)

Каждый элемент массива `updates` представляет собой массив, где первый элемент — код события:

| Код | Формат массива | Описание |
|---|---|---|
| `0` | `[0, $message_id, 0]` | Удаление сообщения с идентификатором `$message_id`. |
| `1` | `[1, $message_id, $flags]` | Замена всех флагов сообщения на `$flags`. |
| `2` | `[2, $message_id, $mask, $user_id]` | Установка флагов сообщения по маске `$mask` (`FLAGS \|= $mask`). |
| `3` | `[3, $message_id, $mask, $user_id]` | Сброс флагов сообщения по маске `$mask` (`FLAGS &= ~$mask`). |
| `4` | `[4, $message_id, $flags, $from_id, $timestamp, $subject, $text, $attachments]` | Добавление нового сообщения. |
| `8` | `[8, -$user_id, 0]` | Друг `$user_id` стал онлайн. |
| `9` | `[9, -$user_id, $flags]` | Друг `$user_id` стал оффлайн (`$flags = 0` — выход с сайта, `1` — таймаут). |

---

### 4. Флаги сообщений (`flags`)

Флаг сообщения формируется как битовая маска (сумма значений):

| Значение | Название | Описание |
|---|---|---|
| `+1` | `UNREAD` | Сообщение не прочитано. |
| `+2` | `OUTBOX` | Исходящее сообщение. |
| `+4` | `REPLIED` | На сообщение был создан ответ. |
| `+8` | `IMPORTANT` | Помеченное (важное) сообщение. |
| `+16` | `CHAT` | Сообщение отправлено из групповой беседы. |
| `+32` | `FRIENDS` | Сообщение отправлено другом. |
| `+64` | `SPAM` | Сообщение помечено как спам. |
| `+128` | `DELETED` | Сообщение удалено (в корзине). |
| `+256` | `FIXED` | Сообщение проверено пользователем на спам. |
| `+512` | `MEDIA` | Сообщение содержит медиавложения. |

---

### 5. Поля объекта вложений (`attachments` в событии 4)

Если в запросе передан `mode=2`, последним элементом события `4` передаётся JSON-объект вложений и метаданных:

| Поле | Пример значения | Описание |
|---|---|---|
| `attach{$i}_type` | `"photo"`, `"video"`, `"audio"`, `"doc"` | Тип `$i`-го вложения (`i >= 1`). |
| `attach{$i}` | `"{owner_id}_{item_id}"` | Идентификатор `$i`-го вложения (`i >= 1`). |
| `fwd` | `"{user_id}_{msg_id},{user_id}_{msg2_id}"` | Идентификаторы пересланных сообщений. |
| `from` | `"12345"` | Идентификатор реального автора сообщения при отправке в беседу. |
| `emoji` | `"1"` | Сообщение содержит эмодзи. |
| `source_act` | `"chat_create"`, `"chat_title_update"` | Сервисное действие в беседе. |
| `source_mid` | `"12345"` | Идентификатор затронутого пользователя для сервисного действия. |