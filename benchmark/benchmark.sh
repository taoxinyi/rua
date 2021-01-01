#!/bin/bash
export CONN=100
export THREADS=4
export DURATION=30

base_url="http://172.31.54.65"
paths=("shortR" "mediumR" "longR" "shortH" "mediumH" "longH")
function average() {
  awk "{ sum += \$$2; n++ } END { if (n > 0) print sum / n; }" "$1"
}

function benchmark() {
  for path in "${paths[@]}"; do
    export URL=$base_url/$path

    echo "Benchmark for $1 conn: $CONN threads: $THREADS duration: $DURATION url: $URL"
    file="metrics_$1.log"
    rm -f "$file"
    bash "$1"/run.sh &
    for i in $(seq 2 $DURATION); do
      sleep 1
      LINES=8 top -b -n 1 -w | tail -n 1 >>"$file"
    done
    sleep 1.5
    # col 9 cpu, 10 memory
    cpu=$(average "$file" 9)
    memory=$(average "$file" 10)
    echo "CPU: $cpu"
    echo "Memory: $memory"
  done

}

if [ "$#" -ne 1 ]; then
  echo "Illegal number of parameters"
  exit 1
fi

if [ "$1" = "all" ]; then
  echo "Benchmark for all"
  for f in *; do
    if [ -d "$f" ]; then
      benchmark "$f"
    fi
  done
else
  benchmark "$1"
fi

exit 0
