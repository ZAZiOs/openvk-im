[***Использовать документацию 2019 года***](https://web.archive.org/web/20190817134247/https://vk.com/dev/messages)

⚠️ - Требуется докинуть longpoll event
ℹ️ - Требует доработки со стороны PHP

### Работа с сообщениями "messages"
* ✅ **messages.send** — отправка сообщения.
* ✅ **messages.edit** — редактирование сообщения.
* ✅ **messages.delete** — удаление сообщения.
* ✅ **messages.restore** — восстановление удаленного сообщения.
* ✅ **messages.search** — поиск по сообщениям пользователя.
* ✅ **messages.getById** — получение сообщений по их ID.
* ✅ **messages.getByConversationMessageId** — получение сообщений по ID внутри конкретной беседы.
* ✅ **messages.markAsRead** — пометка сообщений как прочитанных.
* ✅ **messages.markAsImportant** — пометка сообщений как важных.
* ✅ **messages.getImportantMessages** — получение списка важных сообщений.
* ✅ **messages.pin** — закрепление сообщения.
* ✅ **messages.unpin** — открепление сообщения.

### Работа с беседами (чатами) "chats"
* ℹ️ **messages.createChat** — создание новой беседы. (`PHP` Добавить проверку что у юзера в друзьях есть добавляемые участники)
* ❌ `PHP:` **messages.getChat** — получение информации о беседе.
* ❌ **messages.getChatPreview** — получение данных для превью чата по ссылке. (требует реализации инвайтов)
* ❌ **messages.editChat** — изменение названия беседы.
* ❌ `PHP: (+go lp event)` **messages.setChatPhoto** — установка фотографии беседы.
* ❌ `PHP: (+go lp event)` **messages.deleteChatPhoto** — удаление фотографии беседы.
* ❌ **messages.addChatUser** — добавление пользователя в беседу.
* ❌ **messages.removeChatUser** — исключение пользователя из беседы.
* ✅ **messages.getConversationMembers** — список участников беседы.
* ✅ **messages.getConversations** — список бесед пользователя.
* ✅ **messages.getConversationsById** — получение информации о беседах по ID.
* ℹ️ `PHP:` **messages.searchConversations** — поиск по диалогам и беседам. (На PHP так как нужен поиск по именам)
* ℹ️ **messages.deleteConversation** — удаление всей беседы (истории).
* ✅ **messages.markAsAnsweredConversation** — отметка беседы как «отвеченной».
* ✅ **messages.markAsImportantConversation** — пометка беседы как важной.
* ❌ **messages.getInviteLink** — получение ссылки для приглашения в беседу.
* ❌ **messages.joinChatByInviteLink** — вход в чат по ссылке-приглашению.

### История и медиафайлы "history"
* ✅ **messages.getHistory** — получение истории сообщений диалога.
* ✅ **messages.getHistoryAttachments** — получение медиафайлов (материалов) диалога.

### Статусы и активность "status"
* ✅ `PHP:` **messages.getLastActivity** — дата последней активности пользователя.
* ✅ **messages.setActivity** — передача статуса набора текста («печатает...»).

> ЧИСТО PHP КОД:
### Работа с сообществами (группами) "clubs"
* ❌ **messages.allowMessagesFromGroup** — Разблокировать отправку сообщений от группы. req: group_id, res: 1
* ❌ **messages.denyMessagesFromGroup** — Заблокировать отправку сообщений от группы. req: group_id, res: 1
* ❌ **messages.isMessagesFromGroupAllowed** — Проверить может ли группа тебе писать. req: group_id (от чьего лица) & user_id (кому), res: is_allowed: 0/1

### Long Poll (обновления в реальном времени) "longpoll"
* ✅ **messages.getLongPollServer** — получение данных для подключения к Long Poll.
* ✅ **messages.getLongPollHistory** — получение истории обновлений через Long Poll.

### Custom эндпоинты
* ✅ **im.getCounters** — Возвращает количество unread сообщений.