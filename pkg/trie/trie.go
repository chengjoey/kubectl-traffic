package trie

import "fmt"

type Node interface {
	GetRoute() string
	GetDuration() int64
}

type Trie struct {
	data        map[byte]*Trie
	route       string
	end         bool
	count       int64
	avgDuration float64
}

func NewTrie() *Trie {
	return &Trie{data: make(map[byte]*Trie)}
}

func (t *Trie) Output() string {
	return fmt.Sprintf("count: %d, avgDuration: %f, route: %s", t.count, t.avgDuration, t.route)
}

func (t *Trie) Insert(n Node) {
	route := n.GetRoute()
	dummy := t
	for i := 0; i < len(route); i++ {
		if dat, ok := dummy.data[route[i]]; ok {
			dummy = dat
		} else {
			dummy.data[route[i]] = &Trie{data: make(map[byte]*Trie)}
			dummy = dummy.data[route[i]]
		}
	}
	dummy.end = true
	dummy.route = route
	dummy.count++
	dummy.avgDuration = (dummy.avgDuration*float64(dummy.count-1) + float64(n.GetDuration())) / float64(dummy.count)
}

func (t *Trie) Search(n Node) bool {
	route := n.GetRoute()
	dummy := t
	for i := 0; i < len(route); i++ {
		if dat, ok := dummy.data[route[i]]; ok {
			dummy = dat
		} else {
			return false
		}
	}
	return dummy.end
}

func (t *Trie) GetAll() []*Trie {
	nodes := make([]*Trie, 0)
	for _, v := range t.data {
		if v.end {
			nodes = append(nodes, v)
		}
		nodes = append(nodes, v.GetAll()...)
	}
	return nodes
}

func (t *Trie) TopN(n int) []*Trie {
	nodes := t.GetAll()
	res := make([]*Trie, 0, n)
	for i := len(nodes)/2 - 1; i >= 0; i-- {
		sink(nodes, i, len(nodes))
	}
	for i := 0; i < n && i < len(nodes); i++ {
		res = append(res, nodes[0])
		nodes[0], nodes[len(nodes)-1-i] = nodes[len(nodes)-1-i], nodes[0]
		sink(nodes, 0, len(nodes)-i-1)
	}
	return res
}

func (t *Trie) OutputTopN(n int) string {
	nodes := t.TopN(n)
	res := ""
	for i, node := range nodes {
		res += fmt.Sprintf("%d: %s\n", i+1, node.Output())
	}
	return res
}

func sink(list []*Trie, i int, length int) {
	for {
		idx := i
		l, r := 2*idx+1, 2*idx+2
		if l < length && list[l].count > list[idx].count {
			idx = l
		}
		if r < length && list[r].count > list[idx].count {
			idx = r
		}
		if idx == i {
			break
		}
		list[i], list[idx] = list[idx], list[i]
		i = idx
	}
}
