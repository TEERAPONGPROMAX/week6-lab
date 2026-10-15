
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"moviehub/auth"
	"moviehub/handler"
	"moviehub/mw"
	"moviehub/store"
)

func main() {
	// 1. ตรวจสอบ JWT Secret
	secret := os.Getenv("JWT_SECRET")
	if len(secret) <= 32 {
		log.Fatal("JWT_SECRET ต้องยาวมากกว่า 32 ตัวอักษร")
	}

	// 2. เชื่อมต่อฐานข้อมูล PostgreSQL (Supabase)
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

	// 3. ตั้งค่าระบบ Authentication
	tickets := &auth.Issuer{
		Secret: []byte(secret),
		TTL:    time.Hour,
	}

	// 4. สร้าง Handler
	h := &handler.Handler{
		Store:   &store.Store{Pool: pool},
		Tickets: tickets,
	}

	// 5. กำหนด API Routes
	mux := http.NewServeMux()

	// Authentication
	mux.HandleFunc("POST /api/auth/register", h.Register)
	mux.HandleFunc("POST /api/auth/login", h.Login)

	// Movies
	mux.HandleFunc("GET /api/movies", h.SearchMovies)
	mux.HandleFunc("GET /api/movies/{id}", h.GetMovie)

	// Reviews
	mux.HandleFunc("GET /api/movies/{id}/reviews", h.ListReviews)

	// 6. ตั้งค่า CORS สำหรับ Frontend บน Vercel
	origins := strings.Split(
		os.Getenv("ALLOWED_ORIGINS"),
		",",
	)

	// 7. ตั้งค่า Port สำหรับ Render
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 8. เพิ่ม Logging และ CORS Middleware
	handlerWithMiddleware := mw.Logging(
		mw.CORS(origins, mux),
	)

	// 9. เริ่มต้น Backend Server
	log.Println("MovieHub API ทำงานที่ Port:", port)
	log.Println("Allowed Origins:", origins)

	log.Fatal(
		http.ListenAndServe(
			":"+port,
			handlerWithMiddleware,
		),
	)
}
