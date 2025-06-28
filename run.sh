#!/usr/bin/env bashio

# Check if we're running in Home Assistant environment
if [ -f "/data/options.json" ]; then
    export CLIPPER_CONTACTS_FILE=${CLIPPER_CONTACTS_FILE:-$(jq -r '.contacts_file // empty' /data/options.json 2>/dev/null || echo "")}
    export CLIPPER_PORT=${CLIPPER_PORT:-$(jq -r '.port // empty' /data/options.json 2>/dev/null || echo "8080")}
else
    # Standard Docker environment
    export CLIPPER_CONTACTS_FILE=${CLIPPER_CONTACTS_FILE:-/config/clipper/contacts.json}
    export CLIPPER_PORT=${CLIPPER_PORT:-8080}
fi

# Common settings
export CLIPPER_MEDIA_DIR=${CLIPPER_MEDIA_DIR:-/data/clipper/media}

# Start the server
cd /app
exec ./server 
