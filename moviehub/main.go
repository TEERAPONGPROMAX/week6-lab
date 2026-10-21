
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv/autoload" // โหลด .env ให้เองถ้ามีไฟล์ (ในเครื่อง) บน Render ไม่มีไฟล์ก็เงียบ

	"moviehub/auth"
	"moviehub/handler"
	"moviehub/mw"
	"moviehub/store"
)

func main() {
	
	secret := os.Getenv("JWT_SECRET")
	if len(secret) <= 32 {
		log.Fatal("JWT_SECRET ต้องยาวมากกว่า 32 ตัวอักษร")
	}


	pool, err := pgxpool.New(
		context.Background(),
		os.Getenv("DATABASE_URL"),
	)
	if err != nil {
		log.Fatal("เชื่อมต่อฐานข้อมูลผิดพลาด: ", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("ต่อฐานข้อมูลไม่ติด: ", err)
	}

	
	tickets := &auth.Issuer{
		Secret: []byte(secret),
		TTL:    time.Hour,
	}

	
	h := &handler.Handler{
		Store:   &store.Store{Pool: pool},
		Tickets: tickets,
	}

	// login = ต้องมีตั๋ว, admin = ต้องมีตั๋วและบทบาทเป็น admin (ตรวจตั๋วก่อนเสมอ ค่อยดูบทบาท)
    login := func(next http.HandlerFunc) http.HandlerFunc { return mw.RequireLogin(tickets, next) }
    admin := func(next http.HandlerFunc) http.HandlerFunc { return login(mw.RequireRole("admin", next)) }


	mux := http.NewServeMux()


	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)

	mux.HandleFunc("GET /api/me", login(h.Me))
    mux.HandleFunc("GET /api/genres", h.Genres)

	
	mux.HandleFunc("GET /api/movies", h.SearchMovies)
	mux.HandleFunc("GET /api/movies/{id}", h.GetMovie)

	
	mux.HandleFunc("GET /api/movies/{id}/reviews", h.ListReviews)

	// หลังบ้าน: ทุกเส้นทางใต้ /api/admin ผ่าน admin(...) ไม่มีข้อยกเว้น
    mux.HandleFunc("GET /api/admin/stats", admin(h.AdminStats))
    mux.HandleFunc("GET /api/admin/members", admin(h.AdminMembers))
    mux.HandleFunc("PUT /api/admin/members/{id}/role", admin(h.AdminSetRole))
    mux.HandleFunc("GET /api/admin/movies/{id}", admin(h.AdminGetMovie))
    mux.HandleFunc("POST /api/admin/movies", admin(h.AdminCreateMovie))
    mux.HandleFunc("PUT /api/admin/movies/{id}", admin(h.AdminUpdateMovie))
    mux.HandleFunc("DELETE /api/admin/movies/{id}", admin(h.AdminDeleteMovie))

	
	origins := strings.Split(
		os.Getenv("ALLOWED_ORIGINS"),
		",",
	)

	port := os.Getenv("PORT") // Render ส่งมาให้ ในเครื่องไม่มีก็ใช้ 8080
    if port == "" {
        port = "8080"
    }


	log.Println("MovieHub API ฟังอยู่ที่ :"+port, "ต้อนรับ", origins)
    log.Fatal(http.ListenAndServe(":"+port, mw.Logging(mw.CORS(origins, mux))))
}
