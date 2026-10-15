package db

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite" // драйвер SQLite без CGO
)

type Pal struct {
	ID            int
	PalDexNo      string
	NameRu        string
	NameEn        string
	Element       string
	SkillNameRu   string
	SkillTypeRu   string
	DescriptionRu string
}

type Store struct {
	DB *sql.DB
}

func Open(path string) (*Store, error) {
	sq, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	sq.SetMaxOpenConns(1) // SQLite любит одного писателя
	s := &Store{DB: sq}
	return s, s.init()
}

func (s *Store) init() error {
	_, err := s.DB.Exec(`
CREATE TABLE IF NOT EXISTS pals (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	paldex_no      TEXT,
	name_ru        TEXT NOT NULL,
	name_en        TEXT,
	element        TEXT,
	skill_name_ru  TEXT,
	skill_type_ru  TEXT,
	description_ru TEXT
);
DROP INDEX IF EXISTS idx_pals_dex;
DROP INDEX IF EXISTS idx_name_ru;
CREATE UNIQUE INDEX IF NOT EXISTS idx_pal_key ON pals(paldex_no, name_en);
CREATE INDEX IF NOT EXISTS idx_name_en ON pals(name_en);
CREATE INDEX IF NOT EXISTS idx_name_ru_s ON pals(name_ru);
`)
	return err
}

func (s *Store) Upsert(p Pal) error {
	if strings.TrimSpace(p.NameRu) == "" {
		return nil // без имени пала не опознаешь — пропускаем
	}
	if strings.TrimSpace(p.NameEn) == "" {
		p.NameEn = p.NameRu // иначе пустые строки слипнутся в один конфликт
	}
	_, err := s.DB.Exec(`INSERT INTO pals
		(paldex_no, name_ru, name_en, element, skill_name_ru, description_ru)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(paldex_no, name_en) DO UPDATE SET
			name_ru = excluded.name_ru,
			element = excluded.element,
			skill_name_ru = excluded.skill_name_ru,
			description_ru = excluded.description_ru`,
		p.PalDexNo, p.NameRu, p.NameEn, p.Element, p.SkillNameRu, p.DescriptionRu)
	return err
}

func (s *Store) Search(q string) ([]Pal, error) {
	q = strings.ReplaceAll(q, "-", " ")
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil, nil
	}
	// LIKE в SQLite не понимает русский регистр, поэтому
	// отбираем кандидатов SQL-ом, а фильтруем в Go
	rows, err := s.DB.Query(`
SELECT id, COALESCE(paldex_no,''), name_ru, COALESCE(name_en,''), COALESCE(element,''),
       COALESCE(skill_name_ru,''), COALESCE(skill_type_ru,''), COALESCE(description_ru,'')
FROM pals
ORDER BY paldex_no`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Pal
	for rows.Next() {
		var p Pal
		if err := rows.Scan(&p.ID, &p.PalDexNo, &p.NameRu, &p.NameEn, &p.Element,
			&p.SkillNameRu, &p.SkillTypeRu, &p.DescriptionRu); err != nil {
			return nil, err
		}
		if strings.Contains(strings.ToLower(p.NameRu), q) ||
			strings.Contains(strings.ToLower(p.NameEn), q) {
			out = append(out, p)
		}
	}
	return out, rows.Err()
}

func (s *Store) Count() int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM pals`).Scan(&n)
	return n
}

// Clear удаляет всех палов (для полного переимпорта).
func (s *Store) Clear() error {
	_, err := s.DB.Exec(`DELETE FROM pals`)
	return err
}
