rua -c "$CONN" -t "$THREADS" -d "$DURATION"s "$URL" | tail -n 3 | head -n 1 | awk '{print $3}' &
