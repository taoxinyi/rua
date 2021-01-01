autocannon -j -c "$CONN" -d "$DURATION" --workers "$THREADS" "$URL" | jq '.requests.mean'
