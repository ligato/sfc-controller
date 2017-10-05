#!/bin/bash -l

AGENT_DEVS_DIR="/opt/sfc-controller/dev"
AGENT_PLUGINS_DIR="/opt/sfc-controller/plugins"
ETCD_CFG_FILE="etcd.conf"
KAFKA_CFG_FILE="kafka.conf"
SFC_CFG_FILE="sfc.conf"

echo "Setting up environment..."

if [ ! -d "$AGENT_DEVS_DIR" ]; then
  echo "Config file directory '$AGENT_DEVS_DIR' not found. Creating..."
  mkdir -p -v $AGENT_DEVS_DIR
fi

if [ ! -d "$AGENT_PLUGINS_DIR" ]; then
  echo "Plugins directory '$AGENT_PLUGINS_DIR' not found. Creating..."
  mkdir -p -v $AGENT_PLUGINS_DIR
fi

export PLUGINS_DIR=$AGENT_PLUGINS_DIR
export ETCDV3_CONFIG=$AGENT_DEVS_DIR/$ETCD_CFG_FILE
export KAFKA_CONFIG=$AGENT_DEVS_DIR/$KAFKA_CFG_FILE
export SFC_CONFIG=$AGENT_DEVS_DIR/$SFC_CFG_FILE

echo "Done."
