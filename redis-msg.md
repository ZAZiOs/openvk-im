## Message operations

### send_msg:

```json
{
  "action": "msg.send",
  "data": {
    "chat_id": 1,
    "from_id": 1,
    "msg_type": 1,      // 0: system, 1: text
    "text": "Привет!",
    "attachments": [         // Готовый JSON-массив для хранения в БД
      {"type": "photo", "id": 123},
      {"type": "doc", "id": 456}
    ],
    "reply_to": 0
  }
}
```
system msg:
```json
{
  "action": "msg.send",
  "data": {
    "chat_id": 1,
    "from_id": 1,
    "msg_type": 0,
    "text": "chat_user_kick",
    "attachments": [
      {"type": "audiomsg", "id": 123}
    ],
    "reply_to": 0
  }
}
```

### edit_msg:

```json
{
  "action": "edit_msg",
  "data": {
    "msg_id": 505,
    "chat_id": 2000000001,
    "text": "Обновленный текст",
    "attachments": [
       {"type": "photo", "id": 777} 
    ]
  }
}
```

### delete_msg / restore_msg / pin / unpin / markAsRead

```json
{
  "action": "type_here",
  "data": {
    "operator_id": 1,
    "chat_id": 1,
    "msg_id": 1,
  }
}
```

## Chat operations
### create_chat
```json
{
  "action": "chat.create",
  "data": {
    "title": "Название чата",
    "creator_id": 1,
    "user_ids": [2, 3, 4]
  }
}
```

### add_chat_user / remove_chat_user

```json
{
  "action": "type",
  "data": {
    "chat_id": 2000000001,
    "user_id": 5,
    "operator_id": 1 
  }
}
```

## System operations

```json
{
  "action": "set_activity",
  "data": {
    "chat_id": 1,
    "from_id": 2,
    "type": "typing" // typing, audiomsg, file
  }
}
```