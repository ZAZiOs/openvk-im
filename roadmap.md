[***Использовать документацию 2019 года***](https://web.archive.org/web/20190817134247/https://vk.com/dev/messages)

⚠️ - Требуется докинуть longpoll event

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
* ❌ **messages.createChat** — создание новой беседы.
* ❌ **messages.getChat** — получение информации о беседе.
* ❌ **messages.getChatPreview** — получение данных для превью чата по ссылке.
* ❌ **messages.editChat** — изменение названия беседы.
* ❌ **messages.setChatPhoto** — установка фотографии беседы.
* ❌ **messages.deleteChatPhoto** — удаление фотографии беседы.
* ❌ **messages.addChatUser** — добавление пользователя в беседу.
* ❌ **messages.removeChatUser** — исключение пользователя из беседы.
* ❌ **messages.getConversationMembers** — список участников беседы.
* ✅ **messages.getConversations** — список бесед пользователя.
* ❌ **messages.getConversationsById** — получение информации о беседах по ID.
* ❌ **messages.searchConversations** — поиск по диалогам и беседам.
* ❌ **messages.deleteConversation** — удаление всей беседы (истории).
* ❌ **messages.markAsAnsweredConversation** — отметка беседы как «отвеченной».
* ❌ **messages.markAsImportantConversation** — пометка беседы как важной.

### Ссылки-приглашения "invites"
* ❌ **messages.getInviteLink** — получение ссылки для приглашения в беседу.
* ❌ **messages.joinChatByInviteLink** — вход в чат по ссылке-приглашению.

### История и медиафайлы "history"
* ✅ **messages.getHistory** — получение истории сообщений диалога.
* ✅ **messages.getHistoryAttachments** — получение медиафайлов (материалов) диалога.

### Статусы и активность "status"
* ❌ **messages.getLastActivity** — дата последней активности пользователя.
* ❌ **messages.setActivity** — передача статуса набора текста («печатает...»).

### Работа с сообществами (группами) "clubs"
* ❌ **messages.allowMessagesFromGroup** — разрешить сообщения от группы.
* ❌ **messages.denyMessagesFromGroup** — запретить сообщения от группы.
* ❌ **messages.isMessagesFromGroupAllowed** — проверка, разрешены ли сообщения от группы.

### Long Poll (обновления в реальном времени) "longpoll"
* ✅ **messages.getLongPollServer** — получение данных для подключения к Long Poll.
* ✅ **messages.getLongPollHistory** — получение истории обновлений через Long Poll.

### Custom эндпоинты
* ✅ **im.getCounters** — Возвращает количество unread сообщений.