#!/bin/bash

# turn on bash's job control
set -m

if [ "$MOUNT_PKG" == "goofys" ]
then
  # start syslog before trying to mount volume with goofys
  syslog-ng

  echo "mounting with goofys"
  # mount /graph to s3 bucket
  goofys ${BUCKET_NAME} /dgraph

else
  echo "mounting with s3fs-fuse"
  # First create .passwd-s3fs file container secret details and set owner right
  echo $AWS_ACCESS_KEY_ID:$AWS_SECRET_ACCESS_KEY > ${HOME}/.passwd-s3fs
  chmod 600 ${HOME}/.passwd-s3fs
  #mount /dgraph to s3 bucket
  s3fs $BUCKET_NAME /dgraph
fi

# Check if the bucket is mounted.
# Only start dgraph and lemma-chain when bucket is mounted
if mountpoint -q /dgraph; then

  echo "SUCCESSFULLY MOUNTED"

  echo "changing into mounted bucket volume"
  # change into new mounted directory
  cd /dgraph

  echo "starting dgraph zero server"
  # Start the main dgraph server (zero) and put it in the background
  dgraph zero &

  # wait 10s for server to start
  sleep 10s
  echo "starting dgraph alpha"

  dgraph alpha -p /dgraph/p -w /dgraph/w --lru_mb 2048 --zero localhost:5080 &

  #wait 25s for database to prepare before starting lemma-chain
  sleep 25

  echo "starting lemma-chain..."
  # now run lemma-chain
  exec lemma-chain

  # now we bring the primary process back into the foreground
  # and leave it there
  fg %1
else
  echo "[ERROR:] UNABLE TO MOUNT BUCKET. Try Again..."
fi

exec $@



