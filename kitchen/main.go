package main

import (
    "context"
    "errors"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "kitchen/handler"
    "kitchen/mw"
    "kitchen/store"

    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

    cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
    if err != nil { log.Fatal(err) }
    cfg.MaxConns = 10
    cfg.MaxConnIdleTime = 5 * time.Minute

    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil { log.Fatal(err) }
    defer pool.Close()

    if err := pool.Ping(ctx); err != nil { log.Fatal("ต่อคลังไม่ติด ", err) }
    log.Println("ต่อคลังของติดแล้ว")

    //h := &handler.Handler{Store: store.NewMemoryStore()}
    h := &handler.Handler{Store: &store.PostgresStore{Pool: pool}}

    var app http.Handler = http.TimeoutHandler(h.Routes(), 10*time.Second,
        "ครัวใช้เวลานานเกินไป")
    app = mw.Logging(mw.Recovery(mw.CORS(app)))

    srv := &http.Server{Addr: ":8080", Handler: app}

    go func() {
        if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
            log.Fatal(err)
        }
    }()
    log.Println("ครัวเปิดที่ :8080")

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit                              // ยืนรอสัญญาณปิดร้าน
    log.Println("ได้รับสัญญาณปิดร้าน ไม่รับใบสั่งใหม่แล้ว")

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Println("ปิดไม่ทัน", err)
    }
    log.Println("ปิดร้านเรียบร้อย ลูกค้ากลับหมดแล้ว")
}
