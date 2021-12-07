package mr

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"log"
	"net/rpc"
	"os"
	"sort"
	"strconv"
	"time"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}


//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.

	for {
		reply := CallForTask()

		if reply.Tasktype == MAP {
			data, err := os.ReadFile(reply.Filename)
			if err != nil {
				return
			}
		
			kv := mapf(reply.Filename, string(data))
			storeKeyValuesToTempFile(kv, reply.ReduceWorkers)
			CallForSubmit(reply)
			// Call the map function 
		} else if reply.Tasktype == REDUCE {
			kv := getSortedKeyValuesFromTempFile(reply.Filename)
			kvMap := removeDuplicateKeys(kv)
			runReduceAndStore(reducef, kvMap, outputFileName(reply.Filename))
			os.Remove(reply.Filename)
			CallForSubmit(reply)
		} else if reply.Tasktype == SNOOZE {
			time.Sleep(time.Second)
		} else{
			break
		}
	}

	// uncomment to send the Example RPC to the coordinator.
	// CallExample()

}

func outputFileName(tempFileName string) string {
	var pos int
	fmt.Sscanf(tempFileName, intermediateFilePrefix + "%d", &pos)
	// log.Printf("%d %s",pos, tempFileName)
	return outputFilePrefix + strconv.Itoa(pos)
}
func runReduceAndStore(reducef func(string, []string) string, kvMap map[string][]string, outFile string) {
	file, err := os.OpenFile(outFile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if err  != nil {
		return
	}
	defer file.Close()
	for k, v := range kvMap {
		output := reducef(k, v)
		fmt.Fprintf(file, "%s\n", output)
	}
}
func getSortedKeyValuesFromTempFile(fileName string) []KeyValue {
	f, err := os.Open(fileName)
	if err != nil {
		return nil
	}
	defer f.Close()
	list := []KeyValue{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var key, value string
		fmt.Sscanf(scanner.Text(), "%s %s", &key, &value)
		list = append(list, KeyValue{Key: key, Value: value})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Key < list[j].Key
	})

	return list
}

func removeDuplicateKeys(kv []KeyValue) map[string][]string {
	res := make(map[string][]string)

	for _, item := range kv {
		res[item.Key] = append(res[item.Key], item.Value)
	} 

	return res
}

func storeKeyValuesToTempFile(kv []KeyValue, nReduce int) {
	
	separatedData := make([][]KeyValue, nReduce)
	for _, item := range kv {
		index := ihash(item.Key) % nReduce
		separatedData[index] = append(separatedData[index], item)
	}

	for i, data := range separatedData {
		if len(data) == 0 {
			continue
		}

		tmpFileName := intermediateFilePrefix + strconv.Itoa(i)
	
		file, err := os.OpenFile(tmpFileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
		if err != nil {
			continue
		}
		defer file.Close()
		for _, item := range data {
			fmt.Fprintf(file, "%s %s\n", item.Key, item.Value)
		}

	}
}


func CallForTask() TaskReply {
	args := TaskArgs{os.Getpid()}
	reply := TaskReply{}

	err := call("Coordinator.DemandTask", &args, &reply)

	if !err {
		// log.Printf("Worker process: %v will exit\n", args.WorkerId)
	}
	return reply
}

func CallForSubmit(taskReply TaskReply) {
	args := SubmissionArgs{
		WorkerId: os.Getpid(),
		Filename: taskReply.Filename,
		Tasktype: taskReply.Tasktype,
	}

	reply := SubmissionReply{}

	err := call("Coordinator.SubmitTask", &args, &reply)

	if !err {
		// log.Println("Error sending intermediate results to Coordinator")
	}
}
//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
