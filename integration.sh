#!/bin/bash

readonly SCYLLA_IMAGE=scylladb/scylla:latest



function scylla_up() {
  echo "==> Running docker pull for ${SCYLLA_IMAGE}"
  docker pull ${SCYLLA_IMAGE}
  docker-compose up -d  
}

function scylla_down() {
  echo "==> Stopping Scylla"
  docker compose down
}

function scylla_restart() {
  scylla_down
  scylla_up
}
success=false

function wait_for_db(){
  docker_health scylla
  docker_health scylla-2
  docker_health scylla-3
} 


function docker_health() {
  sleep_time=15
  max_attempts=10
  success=false
  echo "Checking health for: $1"
  while [ $(docker inspect --format "{{json .State.Health.Status }}" $1) != "\"healthy\"" ]; do
    printf "."
    sleep $sleep_time
    attempt_num=$(( attempt_num + 1 ))
    if [ $attempt_num -ge $max_attempts ]; then
      break
    fi
  done
  
  success=true
}
scylla_restart
sleep 60
wait_for_db 


if [ $success = true ]; then
  echo "Connected to scylladb"
  go test ./... -v
  scylla_down
else
  # The command was not successful
  echo "Failed to connect to scylladb"
  scylla_down
  exit 1
fi

