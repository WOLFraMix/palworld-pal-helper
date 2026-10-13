package db

import (
	"database/sql"

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
	// SQLite любит один писатель за раз
	sq.SetMaxOpenConns(1)
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_name_ru ON pals(name_ru);
CREATE INDEX IF NOT EXISTS idx_name_en ON pals(name_en);
`)
	return err
}

func (s *Store) Upsert(p Pal) error {
	_, err := s.DB.Exec(`
INSERT INTO pals (paldex_no, name_ru, name_en, element, skill_name_ru, skill_type_ru, description_ru)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (name_ru) DO UPDATE SET
	paldex_no      = excluded.paldex_no,
	name_en        = CASE WHEN excluded.name_en != '' THEN excluded.name_en ELSE pals.name_en END,
	element        = CASE WHEN excluded.element != '' THEN excluded.element ELSE pals.element END,
	skill_name_ru  = CASE WHEN excluded.skill_name_ru != '' THEN excluded.skill_name_ru ELSE pals.skill_name_ru END,
	skill_type_ru  = CASE WHEN excluded.skill_type_ru != '' THEN excluded.skill_type_ru ELSE pals.skill_type_ru END,
	description_ru = CASE WHEN excluded.description_ru != '' THEN excluded.description_ru ELSE pals.description_ru END`,
		p.PalDexNo, p.NameRu, p.NameEn, p.Element,
		p.SkillNameRu, p.SkillTypeRu, p.DescriptionRu)
	return err
}

func (s *Store) Search(q string) ([]Pal, error) {
	rows, err := s.DB.Query(`
SELECT id, COALESCE(paldex_no,''), name_ru, COALESCE(name_en,''), COALESCE(element,''),
       COALESCE(skill_name_ru,''), COALESCE(skill_type_ru,''), COALESCE(description_ru,'')
FROM pals
WHERE name_ru LIKE ? OR name_en LIKE ?
ORDER BY paldex_no`, "%"+q+"%", "%"+q+"%")
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
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Count() int {
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM pals`).Scan(&n)
	return n
}
