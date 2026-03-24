# POSTGRES SQL HIGH AVAILABILITY

## Overview

- 3 Node (1 Primary, 2 Standby)
- 1 Node for Load Balancer
- All instances use Debian 13 (Trixie)
- Etcd, patroni and Pogres sql on same machine

| IP              | Hostname |
| --------------- | -------- |
| 192.168.100.210 | pg-ha-01 |
| 192.168.100.211 | pg-ha-02 |
| 192.168.100.212 | pg-ha-03 |
| 192.168.100.213 | pg-ha-lb |

## Install etcd

1. Install etcd on every node

```bash
sudo apt update && sudo apt install -y etcd-server etcd-client
```

2. Install golang-cfssl on one node (pg-ha-01)

```bash
sudo apt install -y golang-cfssl
```

3. Create Folder for SSL

```bash
mkdir -p ~/etcd-ssl && cd ~/etcd-ssl
```

4. create SSL config file `ca-config.json`

```json
{
  "signing": {
    "default": {
      "expiry": "87600h"
    },
    "profiles": {
      "etcd": {
        "expiry": "87600h",
        "usages": ["signing", "key encipherment", "server auth", "client auth"]
      }
    }
  }
}
```

5. create SSL file `ca-csr.json`

```json
{
  "CN": "etcd-ca",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [
    {
      "C": "ID",
      "L": "Jakarta",
      "O": "etcd-cluster"
    }
  ]
}
```

6. Create CA certificate

```bash
cfssl gencert -initca ca-csr.json | cfssljson -bare ca
```

7. We well see 3 new files:

```
├── ca-config.json
├── ca.csr
├── ca-csr.json
├── ca-key.pem
└── ca.pem
```

8. Create Certificate config for all node: `pg-ha-01-csr.json`, `pg-ha-02-csr.json`, `pg-ha-03-csr.json`

```json
{
  "CN": "pg-ha-01",
  "hosts": ["pg-ha-01", "192.168.100.210", "127.0.0.1", "localhost"],
  "key": {
    "algo": "rsa",
    "size": 2048
  }
}
```

```json
{
  "CN": "pg-ha-02",
  "hosts": ["pg-ha-02", "192.168.100.211", "127.0.0.1", "localhost"],
  "key": {
    "algo": "rsa",
    "size": 2048
  }
}
```

```json
{
  "CN": "pg-ha-03",
  "hosts": ["pg-ha-03", "192.168.100.212", "127.0.0.1", "localhost"],
  "key": {
    "algo": "rsa",
    "size": 2048
  }
}
```

9. Create Certificate for all node:

```bash
# node 1
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=ca-config.json \
  -profile=etcd \
  pg-ha-01-csr.json | cfssljson -bare pg-ha-01

# node 2
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=ca-config.json \
  -profile=etcd \
  pg-ha-02-csr.json | cfssljson -bare pg-ha-02

# node 3
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=ca-config.json \
  -profile=etcd \
  pg-ha-03-csr.json | cfssljson -bare pg-ha-03
```

10. Distribute SSL files to all nodes:

```
# node 1
/etc/etcd/ssl/
├── ca.pem
├── pg-ha-01-key.pem
└── pg-ha-01.pem

# node 2
/etc/etcd/ssl/
├── ca.pem
├── pg-ha-02-key.pem
└── pg-ha-02.pem

node 3
/etc/etcd/ssl/
├── ca.pem
├── pg-ha-03-key.pem
└── pg-ha-03.pem
```

11. Change owner of SSL files:

```bash
sudo chown etcd:etcd /etc/etcd/ssl/*
sudo chmod 600 /etc/etcd/ssl/*
```

12. Create file etcd env on every node: `/etc/default/etcd`

```bash
ETCD_NAME="pg-ha-01"
ETCD_DATA_DIR="/var/lib/etcd"

# Cluster
ETCD_INITIAL_CLUSTER="pg-ha-01=https://192.168.100.210:2380,pg-ha-02=https://192.168.100.211:2380,pg-ha-03=https://192.168.100.212:2380"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_INITIAL_CLUSTER_TOKEN="etcd-pg-cluster"

# Listen
ETCD_LISTEN_PEER_URLS="https://192.168.100.210:2380"
ETCD_LISTEN_CLIENT_URLS="https://192.168.100.210:2379,https://127.0.0.1:2379"

# Advertise
ETCD_INITIAL_ADVERTISE_PEER_URLS="https://192.168.100.210:2380"
ETCD_ADVERTISE_CLIENT_URLS="https://192.168.100.210:2379"

# SSL
ETCD_CERT_FILE="/etc/etcd/ssl/pg-ha-01.pem"
ETCD_KEY_FILE="/etc/etcd/ssl/pg-ha-01-key.pem"
ETCD_TRUSTED_CA_FILE="/etc/etcd/ssl/ca.pem"
ETCD_CLIENT_CERT_AUTH="true"
ETCD_PEER_CERT_FILE="/etc/etcd/ssl/pg-ha-01.pem"
ETCD_PEER_KEY_FILE="/etc/etcd/ssl/pg-ha-01-key.pem"
ETCD_PEER_TRUSTED_CA_FILE="/etc/etcd/ssl/ca.pem"
ETCD_PEER_CLIENT_CERT_AUTH="true"
```

```bash
ETCD_NAME="pg-ha-02"
ETCD_DATA_DIR="/var/lib/etcd"

ETCD_INITIAL_CLUSTER="pg-ha-01=https://192.168.100.210:2380,pg-ha-02=https://192.168.100.211:2380,pg-ha-03=https://192.168.100.212:2380"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_INITIAL_CLUSTER_TOKEN="etcd-pg-cluster"

ETCD_LISTEN_PEER_URLS="https://192.168.100.211:2380"
ETCD_LISTEN_CLIENT_URLS="https://192.168.100.211:2379,https://127.0.0.1:2379"

ETCD_INITIAL_ADVERTISE_PEER_URLS="https://192.168.100.211:2380"
ETCD_ADVERTISE_CLIENT_URLS="https://192.168.100.211:2379"

ETCD_CERT_FILE="/etc/etcd/ssl/pg-ha-02.pem"
ETCD_KEY_FILE="/etc/etcd/ssl/pg-ha-02-key.pem"
ETCD_TRUSTED_CA_FILE="/etc/etcd/ssl/ca.pem"
ETCD_CLIENT_CERT_AUTH="true"
ETCD_PEER_CERT_FILE="/etc/etcd/ssl/pg-ha-02.pem"
ETCD_PEER_KEY_FILE="/etc/etcd/ssl/pg-ha-02-key.pem"
ETCD_PEER_TRUSTED_CA_FILE="/etc/etcd/ssl/ca.pem"
ETCD_PEER_CLIENT_CERT_AUTH="true"
```

```bash
ETCD_NAME="pg-ha-03"
ETCD_DATA_DIR="/var/lib/etcd"

ETCD_INITIAL_CLUSTER="pg-ha-01=https://192.168.100.210:2380,pg-ha-02=https://192.168.100.211:2380,pg-ha-03=https://192.168.100.212:2380"
ETCD_INITIAL_CLUSTER_STATE="new"
ETCD_INITIAL_CLUSTER_TOKEN="etcd-pg-cluster"

ETCD_LISTEN_PEER_URLS="https://192.168.100.212:2380"
ETCD_LISTEN_CLIENT_URLS="https://192.168.100.212:2379,https://127.0.0.1:2379"

ETCD_INITIAL_ADVERTISE_PEER_URLS="https://192.168.100.212:2380"
ETCD_ADVERTISE_CLIENT_URLS="https://192.168.100.212:2379"

ETCD_CERT_FILE="/etc/etcd/ssl/pg-ha-03.pem"
ETCD_KEY_FILE="/etc/etcd/ssl/pg-ha-03-key.pem"
ETCD_TRUSTED_CA_FILE="/etc/etcd/ssl/ca.pem"
ETCD_CLIENT_CERT_AUTH="true"
ETCD_PEER_CERT_FILE="/etc/etcd/ssl/pg-ha-03.pem"
ETCD_PEER_KEY_FILE="/etc/etcd/ssl/pg-ha-03-key.pem"
ETCD_PEER_TRUSTED_CA_FILE="/etc/etcd/ssl/ca.pem"
ETCD_PEER_CLIENT_CERT_AUTH="true"
```

13. Start etcd on every node and validate cluster:

```bash
sudo systemctl start etcd
sudo systemctl enable etcd

# validate
ETCDCTL_API=3 etcdctl --endpoints=https://192.168.100.210:2379,https://192.168.100.211:2379,https://192.168.100.212:2379 --cacert=/etc/etcd/ssl/ca.pem --cert=/etc/etcd/ssl/pg-ha-01.pem --key=/etc/etcd/ssl/pg-ha-01-key.pem status -w table
```

## Install patroni and postgresql

1. Install postgresql and patroni on every node:

```bash
sudo apt install -y postgresql postgresql-contrib patroni

sudo apt install python3-psycopg2 python3-pip
sudo pip install patroni[dependencies] python-etcd --break-system-packages
```

2. Stop and disable service postgresql, we will use patroni to manage postgresql:

```bash
sudo systemctl stop postgresql
sudo systemctl disable postgresql
```

3. Create config patroni on every node: `/etc/patroni/config.yml`

```yaml
scope: postgres-cluster
namespace: /db/
name: pg-ha-01

restapi:
  listen: 192.168.100.210:8008
  connect_address: 192.168.100.210:8008

etcd3:
  hosts: 192.168.100.210:2379,192.168.100.211:2379,192.168.100.212:2379
  protocol: https
  cacert: /etc/etcd/ssl/ca.pem
  cert: /etc/etcd/ssl/pg-ha-01.pem
  key: /etc/etcd/ssl/pg-ha-01-key.pem

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
    postgresql:
      use_pg_rewind: true
      use_slots: true
      parameters:
        wal_level: replica
        hot_standby: "on"
        max_wal_senders: 10
        max_replication_slots: 10
        wal_log_hints: "on"

  initdb:
    - encoding: UTF8
    - data-checksums

  pg_hba:
    - host replication replicator 192.168.100.0/24 scram-sha-256
    - host all all 192.168.100.0/24 scram-sha-256

  users:
    admin:
      password: "password"
      options:
        - createrole
        - createdb
    replicator:
      password: "password"
      options:
        - replication

postgresql:
  listen: 192.168.100.210:5432
  connect_address: 192.168.100.210:5432
  data_dir: /var/lib/postgresql/patroni
  bin_dir: /usr/lib/postgresql/17/bin
  pgpass: /tmp/pgpass0
  authentication:
    replication:
      username: replicator
      password: "password"
    superuser:
      username: postgres
      password: "password"
    # rewind_user generated by patroni
    rewind:
      username: rewind_user
      password: "password"

tags:
  nofailover: false
  noloadbalance: false
  clonefrom: false
  nosync: false
```

```yaml
scope: postgres-cluster
namespace: /db/
name: pg-ha-02

restapi:
  listen: 192.168.100.211:8008
  connect_address: 192.168.100.211:8008

etcd3:
  hosts: 192.168.100.210:2379,192.168.100.211:2379,192.168.100.212:2379
  protocol: https
  cacert: /etc/etcd/ssl/ca.pem
  cert: /etc/etcd/ssl/pg-ha-02.pem
  key: /etc/etcd/ssl/pg-ha-02-key.pem

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
    postgresql:
      use_pg_rewind: true
      use_slots: true
      parameters:
        wal_level: replica
        hot_standby: "on"
        max_wal_senders: 10
        max_replication_slots: 10
        wal_log_hints: "on"

  initdb:
    - encoding: UTF8
    - data-checksums

  pg_hba:
    - host replication replicator 192.168.100.0/24 scram-sha-256
    - host all all 192.168.100.0/24 scram-sha-256

  users:
    admin:
      password: "password"
      options:
        - createrole
        - createdb
    replicator:
      password: "password"
      options:
        - replication

postgresql:
  listen: 192.168.100.211:5432
  connect_address: 192.168.100.211:5432
  data_dir: /var/lib/postgresql/patroni
  bin_dir: /usr/lib/postgresql/17/bin
  pgpass: /tmp/pgpass0
  authentication:
    replication:
      username: replicator
      password: "password"
    superuser:
      username: postgres
      password: "password"
    rewind:
      username: rewind_user
      password: "password"

tags:
  nofailover: false
  noloadbalance: false
  clonefrom: false
  nosync: false
```

```yaml
scope: postgres-cluster
namespace: /db/
name: pg-ha-03

restapi:
  listen: 192.168.100.212:8008
  connect_address: 192.168.100.212:8008

etcd3:
  hosts: 192.168.100.210:2379,192.168.100.211:2379,192.168.100.212:2379
  protocol: https
  cacert: /etc/etcd/ssl/ca.pem
  cert: /etc/etcd/ssl/pg-ha-03.pem
  key: /etc/etcd/ssl/pg-ha-03-key.pem

bootstrap:
  dcs:
    ttl: 30
    loop_wait: 10
    retry_timeout: 10
    maximum_lag_on_failover: 1048576
    postgresql:
      use_pg_rewind: true
      use_slots: true
      parameters:
        wal_level: replica
        hot_standby: "on"
        max_wal_senders: 10
        max_replication_slots: 10
        wal_log_hints: "on"

  initdb:
    - encoding: UTF8
    - data-checksums

  pg_hba:
    - host replication replicator 192.168.100.0/24 scram-sha-256
    - host all all 192.168.100.0/24 scram-sha-256

  users:
    admin:
      password: "password"
      options:
        - createrole
        - createdb
    replicator:
      password: "password"
      options:
        - replication

postgresql:
  listen: 192.168.100.212:5432
  connect_address: 192.168.100.212:5432
  data_dir: /var/lib/postgresql/patroni
  bin_dir: /usr/lib/postgresql/17/bin
  pgpass: /tmp/pgpass0
  authentication:
    replication:
      username: replicator
      password: "password"
    superuser:
      username: postgres
      password: "password"
    rewind:
      username: rewind_user
      password: "password"

tags:
  nofailover: false
  noloadbalance: false
  clonefrom: false
  nosync: false
```

4. Setting permision for ssl etcd for patroni on every node:

```bash
sudo usermod -aG etcd postgres # add postgres user to etcd group
sudo chown etcd:etcd -R /etc/etcd/
sudo chmod 750 -R /etc/etcd/
```

5. Start patroni on first node and validate cluster:

```bash
sudo systemctl enable patroni
sudo systemctl start patroni

# validate
sudo patronictl -c /etc/patroni/config.yml list
```

6. If first node is primary, start patroni on other nodes:

```bash
sudo systemctl enable patroni
sudo systemctl start patroni

# validate
sudo patronictl -c /etc/patroni/config.yml list
```

## Install HA Proxy

1. Install HA Proxy on LB node:

```bash
sudo apt install -y haproxy
```

2. modify haproxy config: `/etc/haproxy/haproxy.cfg`

```cfg
....# other configurations

#---------------------------------------------------------------------
# Stats page
#---------------------------------------------------------------------
listen stats
    bind *:7000
    mode http
    stats enable
    stats uri /
    stats refresh 5s
    stats show-node
    # stats auth admin:admin123

#---------------------------------------------------------------------
# Primary - read/write (port 5432)
#---------------------------------------------------------------------
listen postgres_primary
    bind *:5432
    mode tcp
    option httpchk GET /primary
    http-check expect status 200
    default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions
    server pg-ha-01 192.168.100.210:5432 check port 8008
    server pg-ha-02 192.168.100.211:5432 check port 8008
    server pg-ha-03 192.168.100.212:5432 check port 8008

#---------------------------------------------------------------------
# Replica - read only (port 5433)
#---------------------------------------------------------------------
listen postgres_replica
    bind *:5433
    mode tcp
    balance roundrobin
    option httpchk GET /replica
    http-check expect status 200
    default-server inter 3s fall 3 rise 2 on-marked-down shutdown-sessions
    server pg-ha-01 192.168.100.210:5432 check port 8008
    server pg-ha-02 192.168.100.211:5432 check port 8008
    server pg-ha-03 192.168.100.212:5432 check port 8008
```
