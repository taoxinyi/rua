bombardier -c "$CONN" -d "$DURATION"s -p r -o j "$URL" | jq ".result.rps.mean"