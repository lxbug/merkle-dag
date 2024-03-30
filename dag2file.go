package merkledag

import (
	"encoding/json"
	"strings"
)

const STEP = 4 //表示哈希值和文件类型在二进制数据中的步长。

// Hash to file
// 根据给定的哈希值和路径，从存储中检索文件。
func Hash2File(store KVStore, hash []byte, path string, hp HashPool) []byte {
	// 根据hash和path， 返回curObjBinary对应的文件, hash对应的类型是tree
	flag, _ := store.Has(hash)
	if flag {
		objBinary, _ := store.Get(hash)
		obj := binaryToObj(objBinary)
		pathArr := strings.Split(path, "\\")
		cur := 1
		return getFileByDir(obj, pathArr, cur, store)
	}
	return nil
}

// 根据给定的目录对象和路径数组递归地查找文件。
// 它首先检查当前路径是否匹配当前对象的链接名称，然后根据对象类型继续递归查找或者获取文件数据。
func getFileByDir(obj *Object, pathArr []string, cur int, store KVStore) []byte {
	if cur >= len(pathArr) {
		return nil
	}
	index := 0
	for i := range obj.Links {
		objType := string(obj.Data[index : index+STEP])
		index += STEP
		objInfo := obj.Links[i]
		if objInfo.Name != pathArr[cur] {
			continue
		}
		switch objType {
		case TREE:
			objDirBinary, _ := store.Get(objInfo.Hash)
			objDir := binaryToObj(objDirBinary)
			ans := getFileByDir(objDir, pathArr, cur+1, store)
			if ans != nil {
				return ans
			}
		case BLOB:
			ans, _ := store.Get(objInfo.Hash)
			return ans
		case LIST:
			objLinkBinary, _ := store.Get(objInfo.Hash)
			objList := binaryToObj(objLinkBinary)
			ans := getFileByList(objList, store)
			return ans
		}
	}
	return nil
}

// 用于处理对象类型为 LIST 的情况，递归地获取列表中的所有文件数据。
func getFileByList(obj *Object, store KVStore) []byte {
	ans := make([]byte, 0)
	index := 0
	for i := range obj.Links {
		curObjType := string(obj.Data[index : index+STEP])
		index += STEP
		curObjLink := obj.Links[i]
		curObjBinary, _ := store.Get(curObjLink.Hash)
		curObj := binaryToObj(curObjBinary)
		if curObjType == BLOB {
			ans = append(ans, curObjBinary...)
		} else { //List
			tmp := getFileByList(curObj, store)
			ans = append(ans, tmp...)
		}
	}
	return ans
}

// 将二进制数据解析为对象结构。
func binaryToObj(objBinary []byte) *Object {
	var res Object
	json.Unmarshal(objBinary, &res)
	return &res
}
