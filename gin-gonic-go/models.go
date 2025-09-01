package main

type Todo struct {
	Id    int    `json:"id,omitempty"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type Paginated[T any] struct {
	Data    []T    `json:"data"`
	Page    int    `json:"page"`
	Size    int    `json:"size"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}
