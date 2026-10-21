package handler

import (
    "encoding/json"
    "errors"
    "net/http"
    "strings"
    "time"
    "unicode/utf8"

    "moviehub/mw"
    "moviehub/store"
)

// ทุก handler ในไฟล์นี้ถูกครอบด้วย RequireLogin + RequireRole("admin") ใน main.go แล้ว
// จึงไม่ต้องเช็กบทบาทซ้ำในนี้ แต่ "ใครเป็นคนกด" ยังอ่านจากตั๋วผ่าน mw.MemberID เสมอ

// ---------- Dashboard ----------

func (h *Handler) AdminStats(w http.ResponseWriter, r *http.Request) {
    st, err := h.Store.Stats(r.Context())
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, st)
}

// ---------- สมาชิก ----------

// AdminMembers (ด่านที่ 5 ของใบงานฝั่ง Go) รายชื่อสมาชิกทั้งหมด
func (h *Handler) AdminMembers(w http.ResponseWriter, r *http.Request) {
    items, err := h.Store.ListMembers(r.Context())
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type roleReq struct {
    Role string `json:"role"`
}

// AdminSetRole เลื่อนหรือลดบทบาท ห้ามแก้ของตัวเอง (กัน admin คนสุดท้ายลดตัวเองจนไม่มีใครเข้าหลังบ้านได้)
func (h *Handler) AdminSetRole(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขสมาชิกต้องเป็นตัวเลข")
        return
    }
    var req roleReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
        return
    }
    if req.Role != "member" && req.Role != "admin" {
        writeError(w, http.StatusBadRequest, "BAD_ROLE", "role ต้องเป็น member หรือ admin")
        return
    }
    if me, _ := mw.MemberID(r.Context()); me == id {
        writeError(w, http.StatusBadRequest, "SELF_ROLE", "เปลี่ยนบทบาทของตัวเองไม่ได้ ให้ admin คนอื่นทำแทน")
        return
    }
    m, err := h.Store.UpdateMemberRole(r.Context(), id, req.Role)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "MEMBER_NOT_FOUND", "ไม่พบสมาชิกคนนี้")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, m)
}

// ---------- แนวหนัง (สาธารณะ ใช้ทำ dropdown) ----------

func (h *Handler) Genres(w http.ResponseWriter, r *http.Request) {
    items, err := h.Store.ListGenres(r.Context())
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ---------- หนัง ----------

type movieReq struct {
    ID int64 `json:"id"` // ใช้เฉพาะตอนสร้าง: ใส่เลข TMDB เองก็ได้ หรือ 0 ให้ระบบออกเลข
    store.MovieInput
}

// trimPtr ตัดช่องว่างหัวท้าย และทำให้ช่องว่างเปล่ากลายเป็น NULL ในฐานข้อมูล
func trimPtr(p *string) *string {
    if p == nil {
        return nil
    }
    s := strings.TrimSpace(*p)
    if s == "" {
        return nil
    }
    return &s
}

// validateMovie ตรวจฝั่ง server ซ้ำอีกชั้น แม้ฟอร์ม React จะเช็กไปแล้ว เพราะ curl ไม่ผ่านฟอร์ม
func validateMovie(in *store.MovieInput) (code, message string) {
    in.Title = strings.TrimSpace(in.Title)
    in.OriginalTitle = strings.TrimSpace(in.OriginalTitle)
    in.OriginalLanguage = strings.ToLower(strings.TrimSpace(in.OriginalLanguage))
    in.TitleTh = trimPtr(in.TitleTh)
    in.Overview = trimPtr(in.Overview)
    in.OverviewTh = trimPtr(in.OverviewTh)
    in.PosterPath = trimPtr(in.PosterPath)
    in.ReleaseDate = trimPtr(in.ReleaseDate)

    switch {
    case in.Title == "" || utf8.RuneCountInString(in.Title) > 255:
        return "BAD_TITLE", "ต้องมีชื่อเรื่อง และยาวไม่เกิน 255 ตัวอักษร"
    case in.OriginalTitle == "":
        in.OriginalTitle = in.Title // ไม่กรอกก็ใช้ชื่อเรื่องแทน
    }
    if in.OriginalLanguage == "" {
        in.OriginalLanguage = "en"
    }
    if len(in.OriginalLanguage) != 2 {
        return "BAD_LANGUAGE", "รหัสภาษาต้องเป็น 2 ตัวอักษร เช่น en, th, ja"
    }
    if in.PosterPath != nil && !strings.HasPrefix(*in.PosterPath, "/") && !strings.HasPrefix(*in.PosterPath, "http") {
        return "BAD_POSTER", "โปสเตอร์ต้องเป็น path ของ TMDB (/xxx.jpg) หรือ URL เต็ม (https://...)"
    }
    if in.ReleaseDate != nil {
        if _, err := time.Parse("2006-01-02", *in.ReleaseDate); err != nil {
            return "BAD_DATE", "วันฉายต้องอยู่ในรูปแบบ YYYY-MM-DD"
        }
    }
    return "", ""
}

func (h *Handler) AdminGetMovie(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
        return
    }
    m, err := h.Store.AdminGetMovie(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, m)
}

func (h *Handler) AdminCreateMovie(w http.ResponseWriter, r *http.Request) {
    var req movieReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
        return
    }
    if req.ID < 0 {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็น 0 (ให้ระบบออก) หรือเลขบวก")
        return
    }
    if code, msg := validateMovie(&req.MovieInput); code != "" {
        writeError(w, http.StatusBadRequest, code, msg)
        return
    }
    m, err := h.Store.CreateMovie(r.Context(), req.ID, req.MovieInput)
    switch {
    case errors.Is(err, store.ErrDuplicateID):
        writeError(w, http.StatusConflict, "MOVIE_EXISTS", "มีหนังเลขนี้อยู่แล้ว")
        return
    case errors.Is(err, store.ErrBadGenre):
        writeError(w, http.StatusBadRequest, "BAD_GENRE", "ไม่มีแนวหนังรหัสนี้")
        return
    case err != nil:
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, m)
}

func (h *Handler) AdminUpdateMovie(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
        return
    }
    var req movieReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
        return
    }
    if code, msg := validateMovie(&req.MovieInput); code != "" {
        writeError(w, http.StatusBadRequest, code, msg)
        return
    }
    m, err := h.Store.UpdateMovie(r.Context(), id, req.MovieInput)
    switch {
    case errors.Is(err, store.ErrNotFound):
        writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
        return
    case errors.Is(err, store.ErrBadGenre):
        writeError(w, http.StatusBadRequest, "BAD_GENRE", "ไม่มีแนวหนังรหัสนี้")
        return
    case err != nil:
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, m)
}

func (h *Handler) AdminDeleteMovie(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
        return
    }
    err := h.Store.DeleteMovie(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    w.WriteHeader(http.StatusNoContent) // ลบแล้ว ไม่มีอะไรให้ส่งกลับ
}