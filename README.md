# Clipper Server

[![Add to Home Assistant](https://my.home-assistant.io/badges/supervisor_addon.svg)](https://my.home-assistant.io/redirect/supervisor_addon/?addon=goldbattle/clipper-server-addon&repository_url=https://github.com/clipper-camera/clipper-server)

This is the server backend for the [Clipper App](https://github.com/clipper-camera/clipper-app) which can be self-hosted to enable the photo and video sharing functionality. The server serves both the contact list to each user along with is a common upload/download location for all users. Checkout the [contacts_example.json](./contacts_example.json) which defines both each user's passwords, names, and friend list they should be served.


Key Features:
- **User Management**: Manages user accounts and relationships through a contacts.json configuration file
- **Media Storage**: Provides a centralized location for storing and serving photos/videos
- **Chat**: Text messages alongside photos and videos, with delivery receipts
- **API Endpoints**:
  - Contact list retrieval for each user
  - File upload/download functionality 
  - Mailbox management for viewing shared media
  - Delivery receipts for messages you sent
  - Health check endpoint


## Messaging

The server is a relay, not a message store. It holds a message only until the
recipient picks it up, then deletes it (10 minutes after pickup, so a flaky
client can retry). Conversation history lives on the devices.

| Endpoint | Purpose |
| --- | --- |
| `POST /_api/v1/upload` | Send a message to one or more of your friends |
| `GET /_api/v1/mailbox/{password}` | List messages waiting for you |
| `GET /_api/v1/download/{password}/{filename}` | Fetch a photo or video |
| `GET /_api/v1/receipts/{password}` | Collect confirmations that your messages arrived |

**Sending.** `POST /_api/v1/upload` is a multipart form taking `userPass`,
`recipients` (a JSON array of user ids, silently filtered to your friends),
`timestamp`, and then either:

- a `media` file part plus `mediaType` of `image` or `video`, or
- a `text` field of up to 4096 bytes, for a chat message.

It responds with `{"success": true, "messageId": "..."}`. One `messageId`
covers every recipient of that send.

**Receiving.** `GET /_api/v1/mailbox/{password}` lists what is waiting, newest
first. Every item carries `id`, `timestamp`, `userId` (the sender) and
`mediaType`. Text messages carry their body inline as `text`; photos and videos
carry a `fileUrl` to `GET` next.

**Delivery receipts.** A message counts as delivered when the recipient
actually takes it: when a text message is handed over in their mailbox listing,
or when a photo or video is downloaded. `GET /_api/v1/receipts/{password}`
returns the confirmations waiting for you:

```json
[{"messageId": "1712345678901234567", "recipientId": 2, "deliveredAt": 1712345699000}]
```

Receipts are held until you collect them, so a delivery that happened while
your app was closed is not lost, and are **deleted once returned** — poll this
endpoint and record the result locally, since a second call will not repeat it.



## How to Run

To run the current code from your go workspace:
```bash
CLIPPER_CONTACTS_FILE="$(pwd)/contacts_example.json" go run ./cmd/clipper-server/main.go
```

To run the server in a Docker container one can do the following:

```bash
# Build the image
docker build -t clipper-server .

# Run the container with custom environment variables
docker run -d \
  -p 8080:8080 \
  -v /path/to/your/contacts.json:/config/clipper/contacts.json \
  -v /path/to/media:/data/clipper/media \
  -e CLIPPER_CONTACTS_FILE=/config/clipper/contacts.json \
  clipper-server
```

After running, you should be able to view the following web pages
- http://localhost:8080/_api/v1/health
- http://localhost:8080/_api/v1/contacts/AAAAA
- http://localhost:8080/_api/v1/mailbox/AAAAA




