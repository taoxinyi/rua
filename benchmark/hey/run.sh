hey -c "$CONN" -z "$DURATION"s "$URL" | head -n 7 | tail -n 1 | awk '{print $2}'
