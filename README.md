Redis High availability is a project
For master/slave

Redis-Master with one more Slave

Use sentinel to manager the nodes.

this process to monitor the sentinel
and then wirte nodes in zookeeper 
for other programming use.


===>SLAVE DOWN<===
1) "pmessage"
2) "*"
3) "+sdown"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"

===>SLAVE UP<===
1) "pmessage"
2) "*"
3) "+reboot"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"
1) "pmessage"
2) "*"
3) "-sdown"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"

=====> Master Down and Switch<====
1) "pmessage"
2) "*"
3) "+sdown"
4) "master mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+odown"
4) "master mymaster 127.0.0.1 7000 #quorum 1/1"

1) "pmessage"
2) "*"
3) "+new-epoch"
4) "7"

1) "pmessage"
2) "*"
3) "+try-failover"
4) "master mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+vote-for-leader"
4) "399711abb0c2933a11d4b265d82c7c40357cc4a7 7"

1) "pmessage"
2) "*"
3) "+elected-leader"
4) "master mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+failover-state-select-slave"
4) "master mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+selected-slave"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+failover-state-send-slaveof-noone"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+failover-state-wait-promotion"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "-role-change"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000 new reported role is master"

1) "pmessage"
2) "*"
3) "+promoted-slave"
4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+failover-state-reconf-slaves"
4) "master mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+slave-reconf-sent"
4) "slave 127.0.0.1:7001 127.0.0.1 7001 @ mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+slave-reconf-inprog"
4) "slave 127.0.0.1:7001 127.0.0.1 7001 @ mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+slave-reconf-done"
4) "slave 127.0.0.1:7001 127.0.0.1 7001 @ mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+failover-end"
4) "master mymaster 127.0.0.1 7000"

1) "pmessage"
2) "*"
3) "+switch-master"
4) "mymaster 127.0.0.1 7000 127.0.0.1 7002"

1) "pmessage"
2) "*"
3) "+slave"
4) "slave 127.0.0.1:7001 127.0.0.1 7001 @ mymaster 127.0.0.1 7002"

1) "pmessage"
2) "*"
3) "+slave"
4) "slave 127.0.0.1:7000 127.0.0.1 7000 @ mymaster 127.0.0.1 7002"

1) "pmessage"
2) "*"
3) "+sdown"
4) "slave 127.0.0.1:7000 127.0.0.1 7000 @ mymaster 127.0.0.1 7002"

===> Master up <===
1) "pmessage"
2) "*"
3) "-role-change"
4) "slave 127.0.0.1:7000 127.0.0.1 7000 @ mymaster 127.0.0.1 7002 new reported role is master"
1) "pmessage"
2) "*"
3) "-sdown"
4) "slave 127.0.0.1:7000 127.0.0.1 7000 @ mymaster 127.0.0.1 7002"
1) "pmessage"
2) "*"
3) "+convert-to-slave"
4) "slave 127.0.0.1:7000 127.0.0.1 7000 @ mymaster 127.0.0.1 7002"
1) "pmessage"
2) "*"
3) "+role-change"
4) "slave 127.0.0.1:7000 127.0.0.1 7000 @ mymaster 127.0.0.1 7002 new reported role is slave"


===> SLAVE ADD <===
1) "pmessage"
2) "*"
3) "+slave"
4) "slave 127.0.0.1:7000 127.0.0.1 7000 @ mymaster 127.0.0.1 7002"
