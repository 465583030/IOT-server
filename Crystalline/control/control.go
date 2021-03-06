package control

import (
	. "Crystalline/conf"
	. "Crystalline/login"
	. "Crystalline/tools"
	. "Crystalline/udp"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/redis.v5"
)

func Login(w http.ResponseWriter, r *http.Request) {
	var flag string
	var mesage string
	var groupnum int64 //用来
	r.ParseForm()      //解析参数
	name := r.PostFormValue("name")
	key := r.PostFormValue("key")
	flag = Getuser(name, key)
	if "no user" == flag {
		fmt.Fprintf(w, " err ")
	} else if "pass" == flag {
		mesage = r.PostFormValue("groupnum")
		a := time.Now().UnixNano()
		randkey := strconv.FormatInt(a, 10)
		member := Client0.ZRangeByScore("2", redis.ZRangeBy{mesage, mesage, 0, 0}).Val() //拿出指令中需要的全部设备ID，后续版本加入同一条指令查询多组的功能

		for num, i := range member { //num是通过数组下标来计数member的，因为从0开始所以需要+1，i是每个deviceid
			Client0.ZAdd(randkey, redis.Z{0, i})
			groupnum = int64(num)
		}
		group_with_offline_num := Client0.ZInterStore(randkey, redis.ZStore{Weight, "sum"}, randkey, "Offline").Val() //将求交集之后的可用device储存在randkey命名的表中，且同时得到可用设备数量
		groupnum = groupnum + 1 - group_with_offline_num                                                              //因为是下标，所以需要+1，然后再减去离线的设备
		Client0.ZAdd("0", redis.Z{float64(groupnum), randkey})                                                        //将randkey为srt写入db0中，记录对应randkey的次数，接受函数需要使用到
		go Findgroup(mesage, randkey)                                                                                 //放在这里比放在上面快多了，groupnum其实和message一样，就是查询一下当前分组有多少个设备
		fmt.Fprintf(w, randkey)                                                                                       //输出到客户端

	}

}
func Watch_dog() {
	for _ = range Ticker.C {
		Client0.ZRemRangeByScore("Offline", "0", "0") //开始之前首先删除上次的离线设备数据
		result := Client0.ZRevRangeByScore("1", redis.ZRangeBy{Min_times, Min_times, 0, 0}).Val()

		//		fmt.Println(result) //历遍数组找出offline设备
		for _, i := range result {
			Client0.ZAdd("Offline", redis.Z{0, i}) //将离线设备进行记录
		}
		//发送report
		resultstr := strings.Join(result, " ")
		crcvalue := Crccal(resultstr)
		packeddata := Code_json(resultstr, "offline_device", crcvalue)
		pack := string(packeddata)
		//		fmt.Println(resultstr) //历遍数组找出offline设备
		resp, err := http.PostForm(API_SEND_SERVER, url.Values{"sysinfo": {pack}})
		if err != nil {
			fmt.Println("离线数据发送失败", err)
			//			Web_alive = false
		} else {
			//			Web_alive = true
			resp.Body.Close()
		}

		devicenumber := Client0.ZCard("1").Val()
		num := Client0.ZRange("1", 0, devicenumber).Val()
		fmt.Println(num)
		for _, mumber := range num {
			Client0.ZAdd("1", redis.Z{0, mumber})

		}
		Getwebalive_flag = Getwebalive(API_AUTO_SERVER) //获取远程端口状态
		predata := Client3.ZCard("Transmit").Val()
		if predata > 0 {
			go Send_store(predata)
		}

		fmt.Printf("ticked at %v ", time.Now())
	}
}

func Send_store(datanum int64) {
	var a int64
	for a = 0; a < datanum; a++ {
		list := Client3.ZRangeByScore("Transmit", redis.ZRangeBy{"1", "1", 0, 1}).Val()
		result := Client3.ZRem("Transmit", list).Val()
		if result == 1 {
			fmt.Println("send store!!!!!")
		}
		for _, i := range list {
			crcvalue := Crccal(i)
			packeddata := Code_json(i, Transmit_randkey, crcvalue)
			pack := string(packeddata)
			resp, err := http.PostForm(API_AUTO_SERVER, url.Values{"data": {pack}})

			if err != nil {
				fmt.Println("Auto_post failed", err)
			} else {
				//发送成功后将Transmit_flag中的flagscore置0表示可以drop
				resp.Body.Close()
			}

		}

	}

}
