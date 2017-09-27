###1. Docker collect 介绍
   docker-collect用于收集机器Docker 容器CPU， 网络，内存，磁盘使用率， 并且把数据发送到graphite时序数据库

###2. 执行的步骤
###2.1 编译和打包docker镜像 
```
$ ./build-docker-image.sh
```

###2.2 执行docker collect 进程
$ docker run -d -v /var/run/docker.sock:/var/run/docker.sock --oom-score-adj=-500 --name docker-collect --net host --restart always docker-collectd:latest /docker-collect  -graphite_url="graphite-host-ip:2003" -stderrthreshold="INFO" -interval=30 -hostname="主机ip地址" -logtostderr
