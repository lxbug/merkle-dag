package merkledag

import (
	"crypto/sha256"
	"encoding/json"
	"hash"
)

const (
	LIST_LIMIT  = 2048
	BLOCK_LIMIT = 256 * 1024
)

const (
	BLOB = "blob"
	LIST = "list"
	TREE = "tree"
)

type Link struct {
	Name string
	Hash []byte
	Size int
}

type Object struct {
	Links []Link
	Data  []byte
}

func Add(store KVStore, node Node, h hash.Hash) []byte {
	// 将节点数据写入到 KVStore 中
	if node.Type() == FILE {
		file := node.(File)                 //类型断言，将node转换为File类型
		tmp := StoreFile(store, file, h)    //将file存储
		jsonMarshal, _ := json.Marshal(tmp) //将tmp对象转换为JSON格式的字节切片
		hash := calculateHash(jsonMarshal)
		return hash
	} else {
		dir := node.(Dir)                   //类型断言，将node转换为Directory类型
		tmp := StoreDir(store, dir, h)      //将dir存储
		jsonMarshal, _ := json.Marshal(tmp) //将tmp对象转换为JSON格式的字节切片
		hash := calculateHash(jsonMarshal)
		return hash
	}

}

// 实现哈希计算的逻辑
func calculateHash(data []byte) []byte {
	h := sha256.New() //创建一个新的SHA256哈希计算器
	h.Write(data)     //将其写入到哈希函数中
	return h.Sum(nil)
}

// 进行文件存储：如果文件大小小于等于256KB，则直接存储在KVStore中；否则，使用merkle dag存储。
func StoreFile(store KVStore, file File, h hash.Hash) *Object {
	if len(file.Bytes()) <= 256*1024 {
		data := file.Bytes()
		blob := Object{Data: data, Links: nil}
		jsonMarshal, _ := json.Marshal(blob) //将其变为json
		hash := calculateHash(jsonMarshal)
		store.Put(hash, data) //将哈希值和文件内容存储到KVStore中
		return &blob          //返回指向blob的内存地址的指针，可以继续使用和修改blob的内容
	}
	linkLen := (len(file.Bytes()) + (256*1024 - 1)) / (256 * 1024)
	hight := 0 //merkle dag 的高度
	tmp := linkLen
	for {
		hight++
		tmp /= 4096
		if tmp == 0 {
			break
		}
	}
	res, _ := dfsForStoreFile(hight, file, store, 0, h)
	return res
}

// 根据文件大小和给定的高度，使用深度优先搜索算法将文件存储在KVStore中，并构建merkle dag。
func dfsForStoreFile(hight int, file File, store KVStore, seedId int, h hash.Hash) (*Object, int) {
	if hight == 1 {
		if (len(file.Bytes()) - seedId) <= 256*1024 {
			data := file.Bytes()[seedId:] //将文件内容从seedId开始截取到末尾，得到剩余数据
			blob := Object{Data: data, Links: nil}
			jsonMarshal, _ := json.Marshal(blob)
			hash := calculateHash(jsonMarshal)
			store.Put(hash, data)
			return &blob, len(data)
		}
		links := &Object{} //创建一个Object结构体实例，用于存储链接
		lenData := 0
		for i := 1; i <= 4096; i++ {
			end := seedId + 256*1024
			if len(file.Bytes()) < end {
				end = len(file.Bytes())
			}
			data := file.Bytes()[seedId:end] //从seedId开始截取到end位置，得到截取的数据
			blob := Object{Data: data, Links: nil}
			lenData += len(data)
			jsonMarshal, _ := json.Marshal(blob)
			hash := calculateHash(jsonMarshal)
			store.Put(hash, data)
			links.Links = append(links.Links, Link{
				Hash: hash,
				Size: len(data),
			})
			links.Data = append(links.Data, []byte("blob")...)
			seedId += 256 * 1024
			if seedId >= len(file.Bytes()) { //如果种子ID大于等于文件大小，则跳出循环
				break
			}
		}
		jsonMarshal, _ := json.Marshal(links)
		hash := calculateHash(jsonMarshal)
		store.Put(hash, jsonMarshal)
		return links, lenData
	} else {
		links := &Object{}
		lenData := 0
		for i := 1; i <= 4096; i++ {
			if seedId >= len(file.Bytes()) {
				break
			}
			tmp, lens := dfsForStoreFile(hight-1, file, store, seedId, h)
			lenData += lens
			jsonMarshal, _ := json.Marshal(tmp)
			hash := calculateHash(jsonMarshal)
			links.Links = append(links.Links, Link{
				Hash: hash,
				Size: lens,
			})
			typeName := "link"
			if tmp.Links == nil {
				typeName = "blob"
			}
			links.Data = append(links.Data, []byte(typeName)...)
		}
		jsonMarshal, _ := json.Marshal(links)
		hash := calculateHash(jsonMarshal)
		store.Put(hash, jsonMarshal)
		return links, lenData
	}
}

// 遍历目录结构，将目录及其下的文件存储到 KVStore 中，并构建一个类似于 Git 中的树状结构，
// 每个目录对应一个树节点，每个文件对应一个叶子节点。
func StoreDir(store KVStore, dir Dir, h hash.Hash) *Object {
	it := dir.It() //遍历目录节点下的所有子节点
	treeObject := &Object{}
	for it.Next() {
		n := it.Node() //当前目录下的node
		switch n.Type() {
		case FILE:
			file := n.(File)
			tmp := StoreFile(store, file, h)
			jsonMarshal, _ := json.Marshal(tmp)
			hash := calculateHash(jsonMarshal)
			treeObject.Links = append(treeObject.Links, Link{
				Hash: hash,
				Size: int(file.Size()),
				Name: file.Name(),
			})
			typeName := "link"
			if tmp.Links == nil {
				typeName = "blob"
			}
			treeObject.Data = append(treeObject.Data, []byte(typeName)...) //将子节点的类型存储到treeObject中
		case DIR:
			dir := n.(Dir)
			tmp := StoreDir(store, dir, h)
			jsonMarshal, _ := json.Marshal(tmp)
			hash := calculateHash(jsonMarshal)
			treeObject.Links = append(treeObject.Links, Link{
				Hash: hash,
				Size: int(dir.Size()),
				Name: dir.Name(),
			})
			typeName := "tree"
			treeObject.Data = append(treeObject.Data, []byte(typeName)...) //将子节点的类型存储到treeObject中
		}
	}
	jsonMarshal, _ := json.Marshal(treeObject)
	hash := calculateHash(jsonMarshal)
	store.Put(hash, jsonMarshal)
	return treeObject
}
