# VK API 5.86 Documentation (Messages)

> Полная официальная спецификация методов секции **`messages`** версии **VK API 5.86** (2019 год).

---

## Содержание

1. [Объекты данных](#объекты-данных)
   - [Message (Сообщение)](#message-сообщение)
   - [Conversation (Беседа / Диалог)](#conversation-беседа--диалог)
   - [ChatSettings (Настройки беседы)](#chatsettings-настройки-беседы)
   - [Chat (Объект чата)](#chat-объект-чата)
2. [Список методов (38)](#список-методов)
   - [Работа с сообщениями](#1-работа-с-сообщениями)
     - [`messages.send`](#messagessend)
     - [`messages.edit`](#messagesedit)
     - [`messages.delete`](#messagesdelete)
     - [`messages.restore`](#messagesrestore)
     - [`messages.markAsRead`](#messagesmarkasread)
     - [`messages.markAsImportant`](#messagesmarkasimportant)
     - [`messages.getById`](#messagesgetbyid)
     - [`messages.getByConversationMessageId`](#messagesgetbyconversationmessageid)
     - [`messages.getImportantMessages`](#messagesgetimportantmessages)
     - [`messages.search`](#messagessearch)
     - [`messages.pin`](#messagespin)
     - [`messages.unpin`](#messagesunpin)
   - [Работа с беседами и диалогами](#2-работа-с-беседами-и-диалогами)
     - [`messages.createChat`](#messagescreatechat)
     - [`messages.getChat`](#messagesgetchat)
     - [`messages.editChat`](#messageseditchat)
     - [`messages.setChatPhoto`](#messagessetchatphoto)
     - [`messages.deleteChatPhoto`](#messagesdeletechatphoto)
     - [`messages.addChatUser`](#messagesaddchatuser)
     - [`messages.removeChatUser`](#messagesremovechatuser)
     - [`messages.getConversationMembers`](#messagesgetconversationmembers)
     - [`messages.getConversations`](#messagesgetconversations)
     - [`messages.getConversationsById`](#messagesgetconversationsbyid)
     - [`messages.searchConversations`](#messagessearchconversations)
     - [`messages.deleteConversation`](#messagesdeleteconversation)
     - [`messages.markAsAnsweredConversation`](#messagesmarkasansweredconversation)
     - [`messages.markAsImportantConversation`](#messagesmarkasimportantconversation)
     - [`messages.getInviteLink`](#messagesgetinvitelink)
     - [`messages.getChatPreview`](#messagesgetchatpreview)
     - [`messages.joinChatByInviteLink`](#messagesjoinchatbyinvitelink)
   - [История сообщений и материалы](#3-история-сообщений-и-материалы)
     - [`messages.getHistory`](#messagesgethistory)
     - [`messages.getHistoryAttachments`](#messagesgethistoryattachments)
   - [Статус и активность](#4-статус-и-активность)
     - [`messages.setActivity`](#messagessetactivity)
     - [`messages.getLastActivity`](#messagesgetlastactivity)
   - [Сообщения сообществ](#5-сообщения-сообществ)
     - [`messages.allowMessagesFromGroup`](#messagesallowmessagesfromgroup)
     - [`messages.denyMessagesFromGroup`](#messagesdenymessagesfromgroup)
     - [`messages.isMessagesFromGroupAllowed`](#messagesismessagesfromgroupallowed)
   - [Long Poll](#6-long-poll)
     - [`messages.getLongPollServer`](#messagesgetlongpollserver)
     - [`messages.getLongPollHistory`](#messagesgetlongpollhistory)

---

## Объекты данных

### Message (Сообщение)

| Поле | Тип | Описание |
|---|---|---|
| `id` | `integer` | Локальный идентификатор сообщения в рамках пользователя/диалога. |
| `date` | `integer` | Время отправки сообщения в формате Unixtime. |
| `peer_id` | `integer` | Идентификатор назначения (ID пользователя, сообщества или `2000000000 + chat_id`). |
| `from_id` | `integer` | Идентификатор автора сообщения. |
| `text` | `string` | Текст сообщения. |
| `random_id` | `integer` | Идентификатор, используемый при отправке сообщения для предотвращения повторной отправки. |
| `out` | `integer (0/1)` | `1` — исходящее сообщение, `0` — входящее сообщение. |
| `important` | `boolean` | `true`, если сообщение помечено как важное. |
| `attachments` | `array` | Массив медиавложений (`photo`, `video`, `audio`, `doc`, `wall`, `sticker`, `gift` и т.д.). |
| `fwd_messages` | `array` | Массив пересланных сообщений (объекты `Message`). |
| `reply_message` | `object` | Сообщение, в ответ на которое отправлено текущее (объект `Message`). |
| `action` | `object` | Сервисное действие в чате (`type`, `member_id`, `text`, `email`, `photo`). |
| `conversation_message_id` | `integer` | Порядковый номер сообщения внутри конкретной беседы. |
| `is_hidden` | `boolean` | `true`, если сообщение скрыто. |
| `update_time` | `integer` | Время последнего редактирования сообщения в Unixtime. |

#### Сервисные действия (`action`):
- `chat_create` — создание беседы (`text` — название беседы)
- `chat_title_update` — изменение названия (`text` — новое название)
- `chat_photo_update` — установка обложки (`photo` — объект фото)
- `chat_photo_remove` — удаление обложки
- `chat_invite_user` — приглашение пользователя (`member_id` — ID приглашённого)
- `chat_kick_user` — исключение пользователя (`member_id` — ID исключённого)
- `chat_pin_message` — закрепление сообщения (`member_id`, `conversation_message_id`)
- `chat_unpin_message` — открепление сообщения
- `chat_invite_user_by_link` — вход по инвайт-ссылке

---

### Conversation (Беседа / Диалог)

| Поле | Тип | Описание |
|---|---|---|
| `peer` | `object` | Информация о собеседнике / диалоге (`id`, `type`: `user`/`chat`/`group`/`email`, `local_id`). |
| `in_read` | `integer` | ID последнего прочитанного входящего сообщения. |
| `out_read` | `integer` | ID последнего прочитанного исходящего сообщения. |
| `unread_count` | `integer` | Число непрочитанных сообщений в диалоге. |
| `important` | `boolean` | `true`, если диалог помечен важным. |
| `unanswered` | `boolean` | `true`, если диалог помечен неотвеченным. |
| `sort_id` | `object` | Объект для сортировки диалогов (`major_id`, `minor_id`). |
| `last_message_id` | `integer` | ID последнего сообщения в диалоге. |
| `chat_settings` | `object` | Настройки беседы (только если `peer.type == 'chat'`). |

---

### ChatSettings (Настройки беседы)

| Поле | Тип | Описание |
|---|---|---|
| `title` | `string` | Название беседы. |
| `members_count` | `integer` | Количество участников в беседе. |
| `state` | `string` | Статус текущего пользователя: `"in"`, `"kicked"`, `"left"`. |
| `photo` | `object` | Обложка беседы (`photo_50`, `photo_100`, `photo_200`). |
| `active_ids` | `array` | Массив идентификаторов последних активных участников. |
| `admin_id` | `integer` | Идентификатор создателя/владельца беседы. |
| `is_group_channel` | `boolean` | `true`, если беседа является каналом сообщества. |
| `pinned_message` | `object` | Закреплённое сообщение (объект `Message`). |
| `acl` | `object` | Права текущего пользователя в беседе. |

#### Объект прав `acl`:
- `can_invite` *(boolean)* — может ли приглашать
- `can_change_info` *(boolean)* — может ли менять название и обложку
- `can_change_pin` *(boolean)* — может ли закреплять сообщения
- `can_promote_users` *(boolean)* — может ли назначать администраторов
- `can_see_invite_link` *(boolean)* — может ли видеть ссылку-приглашение
- `can_change_invite_link` *(boolean)* — может ли перевыпускать ссылку
- `can_moderate` *(boolean)* — может ли исключать участников

---

### Chat (Объект чата)

Используется в `messages.getChat`, `messages.setChatPhoto`, `messages.deleteChatPhoto` и массиве `chats` при `extended=1`:

| Поле | Тип | Описание |
|---|---|---|
| `id` | `integer` | Идентификатор беседы (локальный `chat_id`, от 1 до 100000000). |
| `type` | `string` | Всегда `"chat"`. |
| `title` | `string` | Название беседы. |
| `admin_id` | `integer` | Идентификатор создателя беседы. |
| `users` | `array` | Массив идентификаторов участников (или объектов пользователей при `fields`). |
| `push_settings` | `object` | Настройки уведомлений (`sound`: 1/0, `disabled_until`: int). |
| `photo_50` | `string` | URL копии обложки 50x50. |
| `photo_100` | `string` | URL копии обложки 100x100. |
| `photo_200` | `string` | URL копии обложки 200x200. |
| `left` | `integer` | `1`, если текущий пользователь покинул беседу. |
| `kicked` | `integer` | `1`, если текущий пользователь был исключён. |

---

## Список методов

---

### 1. Работа с сообщениями

#### `messages.send`
Отправляет сообщение пользователю, в беседу или сообщество.

- **Права доступа**: `messages`. Токен пользователя или сообщества.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `user_id` | `integer` | ID получателя (пользователя). |
  | `random_id` | `integer` | **Обязательный.** Уникальный ID для предотвращения дубликатов (32-битное число). |
  | `peer_id` | `integer` | ID назначения (`user_id`, `-group_id` или `2000000000 + chat_id`). |
  | `domain` | `string` | Короткий адрес пользователя/сообщества. |
  | `chat_id` | `integer` | ID беседы. |
  | `user_ids` | `string` | Список ID получателей через запятую (массовая рассылка до 100 человек). |
  | `message` | `string` | Текст сообщения. |
  | `lat` | `number` | Географическая широта (-90..90). |
  | `long` | `number` | Географическая долгота (-180..180). |
  | `attachment` | `string` | Медиавложения через запятую (`<type><owner_id>_<media_id>`). |
  | `reply_to` | `integer` | ID сообщения, на которое создаётся ответ. |
  | `forward_messages` | `string` | Список ID пересылаемых сообщений через запятую. |
  | `forward` | `string` | JSON-объект параметров пересылки (`peer_id`, `conversation_message_ids`, `is_reply`). |
  | `sticker_id` | `integer` | ID стикера. |
  | `group_id` | `integer` | ID сообщества (при вызове с ключом пользователя). |
  | `dont_parse_links` | `integer (0/1)` | Не создавать сниппет ссылки в сообщении. |
- **Результат**: ID отправленного сообщения (`integer`) или массив объектов `[{ peer_id, message_id }]` при `user_ids`.

---

#### `messages.edit`
Редактирует отправленное сообщение.

- **Права доступа**: `messages`. Токен пользователя или сообщества.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID назначения. |
  | `message` | `string` | Новый текст сообщения. |
  | `message_id` | `integer` | Идентификатор сообщения. |
  | `conversation_message_id` | `integer` | Номер сообщения внутри беседы. |
  | `attachment` | `string` | Медиавложения через запятую. |
  | `keep_forward_messages` | `integer (0/1)` | Сохранять ли прикреплённые пересланные сообщения. |
  | `keep_snippets` | `integer (0/1)` | Сохранять ли прикреплённые сниппеты ссылок. |
  | `group_id` | `integer` | ID сообщества. |
  | `dont_parse_links` | `integer (0/1)` | Не создавать сниппет ссылки. |
- **Результат**: `1`.

---

#### `messages.delete`
Удаляет одно или несколько сообщений.

- **Права доступа**: `messages`. Токен пользователя или сообщества.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `message_ids` | `string` | Список идентификаторов сообщений через запятую. |
  | `spam` | `integer (0/1)` | Пометить сообщения как спам. |
  | `delete_for_all` | `integer (0/1)` | `1` — удалить сообщение для всех участников беседы. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: Объект с парами `id: 1` или `0` (успех удаления каждого сообщения).

---

#### `messages.restore`
Восстанавливает удалённое сообщение.

- **Права доступа**: `messages`. Токен пользователя или сообщества.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `message_id` | `integer` | **Обязательный.** ID сообщения. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `1`.

---

#### `messages.markAsRead`
Помечает сообщения как прочитанные.

- **Права доступа**: `messages`. Токен пользователя или сообщества.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `message_ids` | `string` | Список ID сообщений через запятую. |
  | `peer_id` | `integer` | ID назначения (диалога/беседы). |
  | `start_message_id` | `integer` | Номер сообщения, начиная с которого сообщения помечаются прочитанными. |
  | `group_id` | `integer` | ID сообщества. |
  | `mark_conversation_as_read` | `integer (0/1)` | Пометить весь диалог как прочитанный. |
- **Результат**: `1`.

---

#### `messages.markAsImportant`
Помечает сообщения как важные либо снимает отметку.

- **Права доступа**: `messages`. Токен пользователя.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `message_ids` | `string` | Список ID сообщений через запятую. |
  | `important` | `integer (0/1)` | `1` — пометить как важное, `0` — снять отметку. |
- **Результат**: Массив ID успешно обработанных сообщений.

---

#### `messages.getById`
Возвращает сообщения по их глобальным идентификаторам.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `message_ids` | `string` | **Обязательный.** Список ID сообщений через запятую (до 100 штук). |
  | `preview_length` | `integer` | Число символов для обрезки текста (`0` — без обрезки). |
  | `extended` | `integer (0/1)` | `1` — возвращать профили и группы. |
  | `fields` | `string` | Дополнительные поля профилей и групп. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ count: integer, items: [ Message, ... ] }`.

---

#### `messages.getByConversationMessageId`
Возвращает сообщения по их порядковым номерам внутри конкретной беседы.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID назначения. |
  | `conversation_message_ids` | `string` | **Обязательный.** Номера сообщений через запятую (до 100 штук). |
  | `extended` | `integer (0/1)` | `1` — возвращать профили и группы. |
  | `fields` | `string` | Дополнительные поля профилей и групп. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ count: integer, items: [ Message, ... ] }`.

---

#### `messages.getImportantMessages`
Возвращает список важных (отмеченных звёздочкой) сообщений пользователя.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `count` | `integer` | Число сообщений (до 200, по умолчанию 20). |
  | `offset` | `integer` | Смещение. |
  | `start_message_id` | `integer` | ID сообщения, начиная с которого возвращать список. |
  | `preview_length` | `integer` | Длина превью. |
  | `extended` | `integer (0/1)` | `1` — возвращать профили и группы. |
  | `fields` | `string` | Поля профилей. |
- **Результат**: `{ count: integer, items: [ Message, ... ], profiles: [], conversations: [] }`.

---

#### `messages.search`
Поиск по тексту сообщений пользователя.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `q` | `string` | **Обязательный.** Поисковый запрос. |
  | `peer_id` | `integer` | Ограничить поиск конкретным диалогом. |
  | `date` | `string` | Дата отправки в формате `DDMMYYYY`. |
  | `preview_length` | `integer` | Число символов превью. |
  | `offset` | `integer` | Смещение. |
  | `count` | `integer` | Количество сообщений (до 100). |
  | `extended` | `integer (0/1)` | `1` — возвращать профили и группы. |
  | `fields` | `string` | Дополнительные поля. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ count: integer, items: [ Message, ... ] }`.

---

#### `messages.pin`
Закрепляет сообщение в беседе или диалоге.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID назначения. |
  | `message_id` | `integer` | ID закрепляемого сообщения. |
  | `conversation_message_id` | `integer` | Номер сообщения внутри беседы. |
- **Результат**: Объект закреплённого сообщения (объект `Message`).

---

#### `messages.unpin`
Открепляет сообщение в беседе.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID назначения. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `1`.

---

### 2. Работа с беседами и диалогами

#### `messages.createChat`
Создаёт новую беседу с указанными пользователями.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `user_ids` | `string` | **Обязательный.** Список ID участников через запятую. |
  | `title` | `string` | Название создаваемой беседы. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `chat_id` *(integer)* — номер созданной беседы (1..100000000).

---

#### `messages.getChat`
Возвращает подробную информацию о беседе (объект `Chat`).

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `chat_id` | `integer` | Идентификатор беседы. |
  | `chat_ids` | `string` | Список идентификаторов бесед через запятую. |
  | `fields` | `string` | Дополнительные поля профилей участников. |
  | `name_case` | `string` | Падеж для склонения имени и фамилии (`nom`, `gen`, `dat`, `acc`, `ins`, `abl`). |
- **Результат**: Объект `Chat` (при `chat_id`) или массив `[ Chat, ... ]` (при `chat_ids`).

---

#### `messages.editChat`
Изменяет название беседы.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `chat_id` | `integer` | **Обязательный.** Идентификатор беседы. |
  | `title` | `string` | **Обязательный.** Новое название беседы. |
- **Результат**: `1`.

---

#### `messages.setChatPhoto`
Устанавливает фотографию (обложку) беседы.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `file` | `string` | **Обязательный.** Строка, полученная при загрузке на сервер из `photos.getChatUploadServer`. |
- **Результат**: `{ message_id: integer, chat: Chat }`.

---

#### `messages.deleteChatPhoto`
Удаляет фотографию (обложку) беседы.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `chat_id` | `integer` | **Обязательный.** Идентификатор беседы. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ message_id: integer, chat: Chat }`.

---

#### `messages.addChatUser`
Добавляет нового пользователя в беседу.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `chat_id` | `integer` | Идентификатор беседы. |
  | `user_id` | `integer` | **Обязательный.** ID добавляемого пользователя. |
  | `visible_messages_count` | `integer` | Количество отображаемых предыдущих сообщений (0..1000). |
- **Результат**: `1`.

---

#### `messages.removeChatUser`
Исключает пользователя из беседы либо позволяет текущему пользователю выйти из неё.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `chat_id` | `integer` | **Обязательный.** Идентификатор беседы. |
  | `user_id` | `integer` | ID исключаемого пользователя. |
  | `member_id` | `integer` | ID исключаемого участника (в т.ч. отрицательный для сообществ). |
- **Результат**: `1`.

---

#### `messages.getConversationMembers`
Возвращает список участников беседы с подробной информацией о ролях и правах.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID беседы (`2000000000 + chat_id`). |
  | `fields` | `string` | Дополнительные поля профилей участников. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**:
  ```json
  {
    "count": 5,
    "items": [
      {
        "member_id": 1,
        "invited_by": 1,
        "join_date": 1550000000,
        "is_admin": true,
        "can_kick": false
      }
    ],
    "profiles": [ ... ],
    "groups": [ ... ]
  }
  ```

---

#### `messages.getConversations`
Возвращает список бесед и диалогов текущего пользователя или сообщества.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `offset` | `integer` | Смещение для пагинации (по умолчанию `0`). |
  | `count` | `integer` | Количество диалогов (до 200, по умолчанию `20`). |
  | `filter` | `string` | Фильтр диалогов: `all` (все), `unread` (непрочитанные), `important` (важные), `unanswered` (неотвеченные). |
  | `extended` | `integer (0/1)` | `1` — возвращать профили, группы и полные объекты бесед `chats`. |
  | `start_message_id` | `integer` | ID сообщения, начиная с которого возвращать диалоги. |
  | `fields` | `string` | Дополнительные поля профилей. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**:
  ```json
  {
    "count": 10,
    "unread_count": 2,
    "items": [
      {
        "conversation": { /* Conversation object */ },
        "last_message": { /* Message object */ }
      }
    ],
    "profiles": [ ... ],
    "groups": [ ... ]
  }
  ```

---

#### `messages.getConversationsById`
Возвращает информацию о беседах по их `peer_ids`.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_ids` | `string` | **Обязательный.** Список ID диалогов через запятую (до 100). |
  | `extended` | `integer (0/1)` | `1` — возвращать профили и группы. |
  | `fields` | `string` | Дополнительные поля. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ count: integer, items: [ Conversation, ... ] }`.

---

#### `messages.searchConversations`
Поиск по диалогам и беседам пользователя по названию или имени собеседника.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `q` | `string` | Поисковый запрос. |
  | `count` | `integer` | Число результатов (до 100, по умолчанию 20). |
  | `extended` | `integer (0/1)` | `1` — возвращать профили и группы. |
  | `fields` | `string` | Дополнительные поля. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ count: integer, items: [ Conversation, ... ], profiles: [], groups: [] }`.

---

#### `messages.deleteConversation`
Удаляет всю переписку в диалоге или беседе.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | ID назначения (`user_id`, `-group_id` или `2000000000 + chat_id`). |
  | `user_id` | `integer` | ID пользователя (для личных диалогов). |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ last_undeleted_id: integer }` или `1`.

---

#### `messages.markAsAnsweredConversation`
Помечает беседу как отвеченную либо снимает отметку.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID назначения. |
  | `answered` | `integer (0/1)` | `1` — пометить как отвеченный, `0` — снять отметку. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `1`.

---

#### `messages.markAsImportantConversation`
Помечает беседу как важную либо снимает отметку.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID назначения. |
  | `important` | `integer (0/1)` | `1` — пометить важной, `0` — снять отметку. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `1`.

---

#### `messages.getInviteLink`
Получает ссылку для приглашения пользователя в беседу.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID беседы (`2000000000 + chat_id`). |
  | `reset` | `integer (0/1)` | `1` — сгенерировать новую ссылку (аннулировав старую). |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ link: "https://vk.me/join/AJ..." }`.

---

#### `messages.getChatPreview`
Получает данные для превью беседы по ссылке-приглашению (до входа в неё).

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `link` | `string` | **Обязательный.** Ссылка-приглашение (например, `https://vk.me/join/AJ...`). |
  | `fields` | `string` | Дополнительные поля профилей. |
- **Результат**:
  ```json
  {
    "preview": {
      "admin_id": 1,
      "members_count": 12,
      "title": "Беседа друзей",
      "photo": { "photo_50": "...", "photo_100": "...", "photo_200": "..." },
      "local_id": 15
    },
    "profiles": [ ... ],
    "emails": [ ... ]
  }
  ```

---

#### `messages.joinChatByInviteLink`
Позволяет присоединиться к беседе по ссылке-приглашению.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `link` | `string` | **Обязательный.** Ссылка-приглашение. |
- **Результат**: `{ chat_id: integer }`.

---

### 3. История сообщений и материалы

#### `messages.getHistory`
Возвращает историю сообщений для указанного диалога или беседы.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `offset` | `integer` | Смещение для пагинации (по умолчанию `0`). |
  | `count` | `integer` | Количество сообщений (до 200, по умолчанию `20`). |
  | `user_id` | `integer` | ID пользователя. |
  | `peer_id` | `integer` | ID назначения (`user_id`, `-group_id` или `2000000000 + chat_id`). |
  | `start_message_id` | `integer` | ID сообщения, начиная с которого возвращать историю. |
  | `rev` | `integer (0/1)` | `1` — хронологический порядок, `0` — антихронологический *(по умолчанию)*. |
  | `extended` | `integer (0/1)` | `1` — возвращать профили и группы. |
  | `fields` | `string` | Дополнительные поля профилей. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `{ count: integer, items: [ Message, ... ], profiles: [], groups: [] }`.

---

#### `messages.getHistoryAttachments`
Возвращает материалы (фото, видео, аудио, документы, ссылки) из диалога или беседы.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `peer_id` | `integer` | **Обязательный.** ID назначения. |
  | `media_type` | `string` | Тип материалов: `photo`, `video`, `audio`, `doc`, `link`, `market`, `wall`, `share`. По умолчанию `photo`. |
  | `start_from` | `string` | Смещение (cursor), полученное в предыдущем ответе `next_from`. |
  | `count` | `integer` | Количество материалов (до 200, по умолчанию 30). |
  | `photo_sizes` | `integer (0/1)` | Возвращать фото в расширенном формате размеров. |
  | `fields` | `string` | Поля профилей. |
  | `group_id` | `integer` | ID сообщества. |
  | `preserve_order` | `integer (0/1)` | Сохранять порядок. |
  | `max_forwards_level` | `integer` | Глубина поиска во вложенных пересланных сообщениях. |
- **Результат**: `{ items: [ { message_id: int, attachment: { ... } }, ... ], next_from: string }`.

---

### 4. Статус и активность

#### `messages.setActivity`
Передаёт статус активности (набор текста, запись аудиосообщения) в диалог.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `user_id` | `integer` | ID пользователя. |
  | `peer_id` | `integer` | ID диалога или беседы. |
  | `type` | `string` | Тип активности: `typing` (печатает) или `audiomessage` (записывает голосовое). По умолчанию `typing`. |
  | `group_id` | `integer` | ID сообщества. |
- **Результат**: `1`.

---

#### `messages.getLastActivity`
Возвращает дату последней активности и статус «онлайн» пользователя.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `user_id` | `integer` | **Обязательный.** ID пользователя. |
- **Результат**: `{ online: 1/0, time: integer }`.

---

### 5. Сообщения сообществ

#### `messages.allowMessagesFromGroup`
Разрешает отправку сообщений от сообщества текущему пользователю.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `group_id` | `integer` | **Обязательный.** ID сообщества. |
  | `key` | `string` | Произвольный ключ авторизации диалога. |
- **Результат**: `1`.

---

#### `messages.denyMessagesFromGroup`
Запрещает отправку сообщений от сообщества текущему пользователю.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `group_id` | `integer` | **Обязательный.** ID сообщества. |
- **Результат**: `1`.

---

#### `messages.isMessagesFromGroupAllowed`
Проверяет, разрешена ли отправка сообщений от сообщества конкретному пользователю.

- **Права доступа**: `messages`. Токен сообщества или пользователя.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `group_id` | `integer` | **Обязательный.** ID сообщества. |
  | `user_id` | `integer` | **Обязательный.** ID пользователя. |
- **Результат**: `{ is_allowed: 1/0 }`.

---

### 6. Long Poll

#### `messages.getLongPollServer`
Возвращает данные для подключения к Long Poll серверу сообщений.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `need_pts` | `integer (0/1)` | `1` — возвращать `pts`, необходимый для метода `messages.getLongPollHistory`. |
  | `group_id` | `integer` | ID сообщества (для Bots Long Poll). |
  | `lp_version` | `integer` | Версия Long Poll (рекомендуется `3`). |
- **Результат**: `{ key: string, server: string, ts: integer, pts: integer }`.

---

#### `messages.getLongPollHistory`
Возвращает обновления в личных сообщениях пользователя через Long Poll `pts`.

- **Права доступа**: `messages`.
- **Параметры**:
  | Параметр | Тип | Описание |
  |---|---|---|
  | `ts` | `integer` | Последнее значение `ts`. |
  | `pts` | `integer` | Последнее значение `pts`. |
  | `preview_length` | `integer` | Длина превью сообщений. |
  | `onlines` | `integer (0/1)` | Возвращать события онлайна. |
  | `fields` | `string` | Поля профилей. |
  | `events_limit` | `integer` | Лимит событий (по умолчанию 1000). |
  | `msgs_limit` | `integer` | Лимит сообщений (по умолчанию 200). |
  | `max_msg_id` | `integer` | Максимальный ID сообщения в локальной копии. |
  | `group_id` | `integer` | ID сообщества. |
  | `lp_version` | `integer` | Версия Long Poll протокола. |
- **Результат**:
  ```json
  {
    "history": [ ... ],
    "messages": { "count": 2, "items": [ ... ] },
    "profiles": [ ... ],
    "groups": [ ... ],
    "conversations": [ ... ],
    "new_pts": 12345
  }
  ```