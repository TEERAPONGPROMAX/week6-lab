package store

import "context"

type Movie struct {
    ID      int64   `json:"id"`
    Title   string  `json:"title"`
    TitleTh *string `json:"titleTh"`
    Genre   *string `json:"genre"`
    Year    *int    `json:"year"`
    Rating  float64 `json:"rating"`
    Poster  *string `json:"poster"`
}

// SearchMovies ค่าจากผู้ใช้เดินทางผ่าน $1 แยกจากคำสั่งเสมอ
func (s *Store) SearchMovies(ctx context.Context, q string) ([]Movie, error) {
    rows, err := s.Pool.Query(ctx,
        `SELECT m.id, m.title, m.title_th,
                (SELECT coalesce(g.name_th, g.name_en) FROM movie_genres mg
                   JOIN genres g ON g.id = mg.genre_id
                  WHERE mg.movie_id = m.id AND mg.position = 0),
                extract(year FROM m.release_date)::int,
                round(m.vote_average, 1)::float8,
                'https://image.tmdb.org/t/p/w500' || m.poster_path
         FROM movies m
         WHERE lower(m.title) LIKE '%' || lower($1) || '%'
         ORDER BY m.popularity DESC`, q)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := []Movie{}
    for rows.Next() {
        var m Movie
        if err := rows.Scan(&m.ID, &m.Title, &m.TitleTh, &m.Genre, &m.Year, &m.Rating, &m.Poster); err != nil {
            return nil, err
        }
        out = append(out, m)
    }
    return out, rows.Err()
}
