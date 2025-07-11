# Clipper Server

[![Add to Home Assistant](https://my.home-assistant.io/badges/supervisor_addon.svg)](https://my.home-assistant.io/redirect/supervisor_addon/?addon=goldbattle/clipper-server-addon&repository_url=https://github.com/clipper-camera/clipper-server)

This is the server backend for the [Clipper App](https://github.com/clipper-camera/clipper-app) which can be self-hosted to enable the photo and video sharing functionality. The server serves both the contact list to each user along with is a common upload/download location for all users. Checkout the [contacts_example.json](./contacts_example.json) which defines both each user's passwords, names, and friend list they should be served.


Key Features:
- **User Management**: Manages user accounts and relationships through a contacts.json configuration file
- **Media Storage**: Provides a centralized location for storing and serving photos/videos
- **API Endpoints**:
  - Contact list retrieval for each user
  - File upload/download functionality 
  - Mailbox management for viewing shared media
  - Health check endpoint



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




