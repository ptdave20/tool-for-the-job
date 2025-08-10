package main

type Todo struct {
	Id    int    `json:"id,omitempty"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}
