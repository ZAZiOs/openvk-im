## Message operations

### Отправка сообщения

```json
{
  "action": "msg.send",
  "data": {
    "peer_id": 2000000001,
    "from_id": 1,
    "text": "Привет!",
    "attachments": {
      "attach1_type": "photo",
      "attach1": "{owner_id}_{item_id}",
      "fwd": "{$user_id}_{$msg_id}{$user_id}_{$msg2_id}",
      "geo": "geo_id",
      "attach2_product_id": "sticker_id",
      "emoji": 1
    },
    "reply_to": 0
  }
}
```

### edit_msg:

```json
{
  "action": "msg.edit",
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