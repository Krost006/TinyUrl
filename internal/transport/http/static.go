package http

import (
	"net/http"
	"path/filepath"
)

// StaticHandler раздаёт файлы фронтенда: страницы и статику.
type StaticHandler struct {
	dir string
}

// NewStaticHandler принимает каталог, внутри которого лежат index.html
// и остальные страницы.
func NewStaticHandler(dir string) *StaticHandler {
	return &StaticHandler{dir: dir}
}

// pages — страницы, которые сервис отдаёт по прямому пути. Список явный,
// а не FileServer по всему каталогу: иначе любой файл рядом стал бы доступен
// снаружи, а неизвестный путь отдавал бы файловый 404 вместо нашего.
var pages = []string{
	"login.html",
	"register.html",
	"dashboard.html",
}

func (h *StaticHandler) Routes(mux *http.ServeMux) {
	// {$} означает "ровно корень": без него шаблон "/" ловил бы все запросы,
	// не подошедшие другим маршрутам.
	mux.HandleFunc("GET /{$}", h.page("index.html"))

	for _, name := range pages {
		mux.HandleFunc("GET /"+name, h.page(name))
	}

	mux.Handle("GET /static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir(filepath.Join(h.dir, "static")))))
}

func (h *StaticHandler) page(name string) http.HandlerFunc {
	// Имя приходит из pages, не из запроса, поэтому выйти за каталог нельзя.
	path := filepath.Join(h.dir, "static", name)

	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}
