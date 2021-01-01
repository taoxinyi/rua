wrk2 -c "$CONN" -t "$THREADS" -d "$DURATION" -R 250000 "$URL" | tail -n 2 | head -n 1 | awk '{print $2}'
