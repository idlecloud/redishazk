package main

// Redis, zookeeper, config, and log library is need
import (
    "os"
    "fmt"
    "flag"
    "path"
    "time"
    "net"
    "errors"
    "strings"
    "container/list"
    "encoding/json"
    log "github.com/cihub/seelog"
    "github.com/larspensjo/config"
    "github.com/samuel/go-zookeeper/zk"
    redis "github.com/garyburd/redigo/redis"
)

const (
    S_SEC_PUBCONF = "public"
)

const (
    SC_SEELOG_CFG_FILE = "seelogcfgfile"
    SC_HOST = "host"
    SC_ZK_SRV  = "zoo_srv"
    SC_ZK_PATH_M = "zoo_path_master"
    SC_ZK_PATH_S = "zoo_path_slave"
    SC_WEIGHT = "weight"
    SC_NODELIST = "nodelist"
    RECOVERY_TYPE_ZOO = 1
    RECOVERY_TYPE_REDIS = 2
)

type HANodeSt struct {
    Name string
    Master string
    Slaves *list.List
    ZooMaster string
    ZooSlave  string
    Weight    string
}

type NodeValueSt struct {
    Weight string
}

func (r * HANodeSt) DelSlave(node string) {
    for e := r.Slaves.Front(); e != nil ; e = e.Next() {
        if e.Value.(string) == node {
            r.Slaves.Remove(e)
        }
    }
}

func (r * HANodeSt) AddSlave(node string) {
    e := r.Slaves.Front()
    for ; e != nil ; e = e.Next() {
        if e.Value.(string) == node {
            break
        }
    }
    if e == nil {
        r.Slaves.PushBack(node)
    }
}

var (
    gSentinelHost string
    gRedisPool * redis.Pool
    gZKConn * zk.Conn
    gZKSrv  string
    gHANodeMap = make(map[string]*HANodeSt)
    gRecoveryChan = make(chan int, 2)
)

func ParseConfig(cfg *config.Config) {
    gSentinelHost, _ = cfg.String(S_SEC_PUBCONF, SC_HOST)
    gZKSrv, _  = cfg.String(S_SEC_PUBCONF, SC_ZK_SRV)
    biStr, _ := cfg.String(S_SEC_PUBCONF, SC_NODELIST)
    biList := strings.Split(biStr, ",")
    for i := 0; i < len(biList); i++ {
        node := new(HANodeSt)
        node.Name = biList[i]
        node.Weight, _ = cfg.String(biList[i], SC_WEIGHT )
        node.ZooMaster, _ = cfg.String(biList[i], SC_ZK_PATH_M)
        node.ZooSlave, _ = cfg.String(biList[i], SC_ZK_PATH_S)
        gHANodeMap[biList[i]] = node
    }
}

func InitSeeLog(seeLogCfgFile string){
    logger, err := log.LoggerFromConfigAsFile(seeLogCfgFile)
    if err != nil {
        log.Errorf("Parse seelog config file error: %s", err.Error)
        os.Exit(1)
    }
    log.ReplaceLogger(logger)
}

func GetRedisMaster(node *HANodeSt){
    redisConn := gRedisPool.Get()
    defer redisConn.Close()
    values, err := redis.Strings(redisConn.Do("SENTINEL", "get-master-addr-by-name", node.Name ))

    if err != nil {
        log.Errorf("get master error:%s", err.Error())
        return
    }

    str := values[0] + ":" + values[1]
    node.Master = str
    //gHANodeMap[item] = node
}

func GetRedisSlaves(node *HANodeSt) {
    redisConn := gRedisPool.Get()
    defer redisConn.Close()
    //values, err := redis.Strings(redisConn.Do("SENTINEL", "slaves", "mymaster" ))
    //values, err := redisConn.Do("SENTINEL", "slaves", "mymaster" )
    //values, err := redisConn.Do("get", "hello1" )
    values, err := redis.Values(redisConn.Do("SENTINEL", "slaves", node.Name ))

    if err != nil {
        log.Errorf("get master error:%s", err.Error())
    }

    slaveNo := len(values)
    slaveList := list.New()
    for i := 0; i < slaveNo; i++ {
        smap, _ := redis.StringMap(values[i], nil)
        host := smap["ip"]
        port := smap["port"]
        flag := smap["flags"]
        if flag != "slave" {
            continue
        }
        slaveList.PushBack( host + ":" + port )
    }
    node.Slaves = slaveList
    //gHANodeMap[item] = node
}
    /*
    switch values[0].(type){
        case bool:
            fmt.Printf(">>>BOOL\n")
        case []byte:
            fmt.Printf(">>>Bytes\n")
        case float64:
            fmt.Printf(">>>float64\n")
        case int:
            fmt.Printf(">>>int\n")
        case int64:
            fmt.Printf(">>>int64\n")
        case map[string]int64:
            fmt.Printf(">>>map[string]i64\n")
        case map[string]int:
            fmt.Printf(">>>map[string]int\n")
        case map[string]string:
            fmt.Printf(">>>map[string]string\n")
        case []int:
            fmt.Printf(">>>[]int\n")
        case uint64:
            fmt.Printf(">>>uint64\n")
        case string:
            fmt.Printf(">>>string\n")
        case []string:
            fmt.Printf(">>>[]string\n")
        case []interface{}:
            fmt.Printf(">>>interfaces\n")
        default:
            fmt.Printf(">>>default\n")
    }
    fmt.Printf("=====>%s\n", len(values))
    ilen := len(values)
    for i := 0; i < ilen; i++ {
        fmt.Printf(">>%s\n",values[i])
    }
    */

func ZKDial(network, address string, timeout time.Duration) (net.Conn, error){
    conn, err := net.DialTimeout(network, address, timeout)
    if err != nil {
        log.Criticalf("Zookeeper connect [%s] error:%s", address, err.Error())
    } else {
        log.Info("Zookeeper connect sucess")
        select {
        case gRecoveryChan <- RECOVERY_TYPE_ZOO:
            log.Errorf("Chnnanl unblock write event")
        default:
            log.Errorf("Chnnanl block skip event")
        }
    }
    return conn, err
}


func RedisPoolConn() *redis.Pool {
    return &redis.Pool{
        MaxIdle: 10,
        MaxActive: 20, // max number of connections
        Dial: func() (redis.Conn, error) {
            c, err := redis.Dial("tcp", gSentinelHost)
            if err != nil {
                log.Criticalf("redis connect %s err=%s", gSentinelHost, err.Error())
            }
            return c, err
        },
    }
}

func ZKConnect() * zk.Conn {
    zks := strings.Split(gZKSrv, ",")
    conn,_,err := zk.ConnectWithDialer(zks, time.Second, ZKDial)

    if err != nil {
        log.Errorf("Zookeeper connect error=%s", err.Error())
        return nil
    }
    return conn
}

func CreateRecursivePath(zooPath string) error {
    // check if parent dir is exist
    log.Debugf("Create zoo path [%s]", zooPath)
    if zooPath == "/" {
        return nil
    }else{
        //create directly
        bExist, _, err := gZKConn.Exists(zooPath)
        if err != nil {
            log.Errorf("Zookeeper execute Exists error=%s", err.Error())
            return errors.New("Zookeeper Exists path error")
        }
        if bExist == false {
            parentDir := path.Dir(zooPath)
            err := CreateRecursivePath(parentDir)
            if err != nil {
                return err
            }
            // Create it selft
            flags := int32(0)
            acl := zk.WorldACL(zk.PermAll)
            _, err = gZKConn.Create(zooPath, []byte("1"), flags, acl)
            if err != nil {
                log.Errorf("Zookeeper create path [%s] error=%s", zooPath, err.Error())
                return errors.New("Zookeeper create path error")
            }
            return nil
        }else{
            return nil
        }
    }

    return nil
}


func CreateAllNodes(node *HANodeSt){
    flags := int32(zk.FlagEphemeral)
    acl := zk.WorldACL(zk.PermAll)

    masterPath := node.ZooMaster + "/" + node.Master

    stValue := NodeValueSt{Weight: node.Weight}
    bValue, _ := json.Marshal(stValue)

    gZKConn.Create(masterPath, bValue, flags, acl)

    for e := node.Slaves.Front(); e != nil; e = e.Next() {
        slavePath := node.ZooSlave + "/" + e.Value.(string)
        gZKConn.Create(slavePath, []byte("1"), flags, acl)
    }
}

/*
 * PMessage {Pattern string, Channel string, Data []byte}
 * <instance-type> <name> <ip> <port> @ <master-name> <master-ip> <master-port>
 *
 * Slave Down
    1) "pmessage"
    2) "*"
    3) "+sdown"
    4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"
 * Slave up
    1) "pmessage"
    2) "*"
    3) "-sdown"
    4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"
 * Master Down
    1) "pmessage"
    2) "*"
    3) "+switch-master"
    4) "mymaster 127.0.0.1 7000 127.0.0.1 7002"
 * Slave New
    1) "pmessage"
    2) "*"
    3) "+slave"
    4) "slave 127.0.0.1:7002 127.0.0.1 7002 @ mymaster 127.0.0.1 7000"
    
 */
func ParsePMessage(PMsg redis.PMessage) {
    argArr := strings.Split(string(PMsg.Data), " ")
    if PMsg.Channel == "+sdown" && argArr[0] == "slave" {
        // Slave Down
        haNode := gHANodeMap[argArr[5]]
        nodePath := argArr[2] + ":" + argArr[3]
        zooPath := haNode.ZooSlave + "/" + nodePath
        log.Infof("%s (zoopath:%s) down delete from zookeeper", nodePath, zooPath)
        // delete from list
        haNode.DelSlave(nodePath)
        DeleteZooNode(zooPath)
    } else if PMsg.Channel == "-sdown" && argArr[0] == "slave" {
        // Slave Up
        haNode := gHANodeMap[argArr[5]]
        nodePath := argArr[2] + ":" + argArr[3]
        zooPath := haNode.ZooSlave + "/" + nodePath
        log.Infof("%s (zoopath:%s) up create new zookeeper", nodePath, zooPath)

        CreateZooEphemeralNode(zooPath, []byte("1"))
        haNode.AddSlave(nodePath)
    } else if PMsg.Channel == "+slave" && argArr[0] == "slave" {
        // Slave Add
        haNode := gHANodeMap[argArr[5]]
        nodePath := argArr[2] + ":" + argArr[3]
        zooPath := haNode.ZooSlave + "/" + nodePath
        log.Infof("%s (zoopath:%s) up create new zookeeper", nodePath, zooPath)

        CreateZooEphemeralNode(zooPath, []byte("1"))
        haNode.AddSlave(nodePath)
    } else if PMsg.Channel == "+switch-master" {
        // delete master node
        haNode := gHANodeMap[argArr[0]]
        oldNode := argArr[1] + ":" + argArr[2]
        oldMaster := haNode.ZooMaster + "/" + oldNode
        // create new node
        newNode := argArr[3] + ":" + argArr[4]

        haNode.Master = newNode
        newMaster := haNode.ZooMaster + "/" + newNode
        // delete old slave
        oldSlave := haNode.ZooSlave + "/" + newNode

        DeleteZooNode(oldMaster)

        // Value of Json
        stValue := NodeValueSt{Weight: haNode.Weight}
        bValue, _ := json.Marshal(stValue)

        CreateZooEphemeralNode(newMaster, bValue)
        DeleteZooNode(oldSlave)
        log.Infof("Switch delete %s, %s, create %s", oldMaster, oldSlave, newMaster)
    }

    for item := range gHANodeMap {
        haNode := gHANodeMap[item]
        log.Infof("%s Master=%s\n", item, haNode.Master)
        for e := haNode.Slaves.Front(); e != nil; e = e.Next() {
            log.Infof("%s Slave=%s\n", item,  e.Value.(string))
        }
    }
}

func CreateZooEphemeralNode(zooPath string, value []byte) {
    flags := int32(zk.FlagEphemeral)
    acl := zk.WorldACL(zk.PermAll)
    _, err := gZKConn.Create(zooPath, value, flags, acl)
    if err != nil {
        log.Criticalf("Create Zookeeper Node error=%s", err.Error())
    }
}

func DeleteZooNode(zooPath string){
    gZKConn.Delete(zooPath, 0)
}

// Monitor sentinel
func MonitorSentinel(){
    redisConn := gRedisPool.Get()
    defer redisConn.Close()

    psc := redis.PubSubConn{redisConn}
    psc.PSubscribe("*")
    runflag := true
    for runflag {
        switch v := psc.Receive().(type){
            case redis.Message:
                log.Infof("Type Message>>channel %s, message: %s", v.Channel, v.Data)
            case redis.Subscription:
                log.Infof("Type Subscribe>>channel %s, kind %s, count %d", v.Channel, v.Kind, v.Count)
                gRecoveryChan <- RECOVERY_TYPE_REDIS
            case error:
                log.Error("MonitorSentinel ERROR")
                runflag = false
                // Should re psubscrebe
            case redis.PMessage:
                log.Infof("Type PMessage>>channel %s, pattern %s, data %s", v.Channel, v.Pattern, v.Data)
                ParsePMessage(v)
            default:
                log.Warnf("Unkown Message Type of psubscribe")
        }
    }
}

func CreateMasterZooNode(haNode * HANodeSt){
    newMaster := haNode.ZooMaster + "/" + haNode.Master
    stValue := NodeValueSt{Weight: haNode.Weight}
    bValue, _ := json.Marshal(stValue)

    CreateZooEphemeralNode(newMaster, bValue)
}

func CreateSlaveZooNode(haNode * HANodeSt, slave string) {
    newSlave := haNode.ZooSlave + "/" + slave
    stValue := NodeValueSt{Weight: haNode.Weight}
    bValue, _ := json.Marshal(stValue)

    CreateZooEphemeralNode(newSlave, bValue)
}

// if zoo, need re create all node
func Recovery(zooFlag bool){
    // For each HA 1. Get master and check if need create zoo node, 
    // 2 Get Slaves and check if need create zoo node
    // delete the Old Slaves
    for item := range gHANodeMap {
        log.Infof("Start recovery master of %s", item)
        haNode := gHANodeMap[item]

        // ------------ Master ------------
        oldMaster := haNode.Master
        // Get master
        GetRedisMaster(haNode)
        log.Infof("Recovery get master [%s] of %s", haNode.Master, item)
        if zooFlag {
            // Create Master
            CreateMasterZooNode(haNode)
            log.Infof("Zookeeper reconnect master create %s", oldMaster );
        }else {
            if oldMaster != haNode.Master { // equal skip
                // delete
                oldPath := haNode.ZooMaster + "/" + oldMaster
                DeleteZooNode(oldPath)
                // create
                CreateMasterZooNode(haNode)
                log.Infof("Redis reconnect swith master delete %s, create %s", oldPath, haNode.Master)
            }else{
                log.Infof("Redis reconnect dothing of %s", item)
            }
        }

        // ---------- Slave --------------
        log.Infof("Start recovery slave of %s", item)
        oldSlaveList := haNode.Slaves;
        GetRedisSlaves(haNode)
        log.Infof("Start recovery slave of %s get %d node", item, haNode.Slaves.Len())
        if zooFlag == false {
            // change list to map
            rNodeMap := make(map[string]int)
            for e := oldSlaveList.Front(); e != nil; e = e.Next() {
                rNodeMap[e.Value.(string)] = 1
            }

            for e := haNode.Slaves.Front(); e != nil; e = e.Next() {
                sNode := e.Value.(string)
                _, ok := rNodeMap[sNode]
                if ok {
                    log.Infof("Redis Reconn The slave %s is in old list", sNode)
                    // delete from map
                    delete(rNodeMap, sNode)
                }else
                {
                    log.Infof("Redis Reconn The slave %s is note in old list create it", sNode)
                    CreateSlaveZooNode(haNode, sNode)
                }
            }

            // delete the remain in Map
            for k, _ := range rNodeMap {
                log.Infof("Redis Reconn The slave [%s] not exist in new list delete it", k)
                oldPath := haNode.ZooSlave + "/" + k
                DeleteZooNode(oldPath)
            }
        }else {
            for e := oldSlaveList.Front(); e != nil; e = e.Next() {
                sNode := e.Value.(string)
                CreateSlaveZooNode(haNode, sNode)
                log.Infof("Zookeeper reconnect create %s", sNode)
            }
        }
    }
}

func RecoveryMain(){
    zooSkipFlag := true;
    redisSkipFlag := true;
    for recovType := range gRecoveryChan {
        if recovType == RECOVERY_TYPE_ZOO {
            if zooSkipFlag {
                zooSkipFlag = false
                continue
            }
            time.Sleep(1e9)
            log.Errorf("Zoo reconnect recovery start")
            Recovery(true)
            log.Errorf("Zoo reconnect recovery finish")
        }else if recovType == RECOVERY_TYPE_REDIS {
            if redisSkipFlag {
                redisSkipFlag = false
                continue
            }
            log.Errorf("Sentinel reconnect recovery")
            Recovery(false)
        }
    }
}

func main(){
    flag.Parse()
    cfgFile := flag.String("conf", "redis_ha.conf", "General configuration file")

    var cfg *config.Config
    cfg, err := config.ReadDefault(*cfgFile)
    if err != nil {
        log.Criticalf("Fail to find", *cfgFile, err)
    }

    // Get the logfile config
    logCfgFile, _ := cfg.String(S_SEC_PUBCONF, SC_SEELOG_CFG_FILE)
    InitSeeLog(logCfgFile)
    log.Info("REDIS HIGH AVAILABITY START")

    // Parse the config
    ParseConfig(cfg)

    log.Infof("Sentinel host: %s", gSentinelHost)
    // Get the master, and slave
    gRedisPool = RedisPoolConn()

    // create zknodes
    gZKConn = ZKConnect()

    for item := range gHANodeMap {
        haNode := gHANodeMap[item]

        GetRedisMaster(haNode)
        GetRedisSlaves(haNode)

        fmt.Printf("%s Master=%s\n", item, haNode.Master)
        for e := haNode.Slaves.Front(); e != nil; e = e.Next() {
            fmt.Printf("Slave=%s\n", e.Value.(string))
        }

        CreateRecursivePath(haNode.ZooMaster)
        CreateRecursivePath(haNode.ZooSlave)

        CreateAllNodes(haNode)
    }

    go RecoveryMain()
    MonitorSentinel()
    for {
        /*
        gConn, err = redis.Dial("tcp", gSentinelHost)
        if err != nil {
            log.Criticalf("Connect to sentinel error:%s", err.Error())
            continue
        }
        */
        MonitorSentinel()
        time.Sleep(1e9)
    }

    defer gZKConn.Close()
}
