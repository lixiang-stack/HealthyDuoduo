#!/usr/bin/env bash
# Seed the isolated Android acceptance stack with synthetic data:
# one member, two growth records, two dated liver reports, one synthetic clinical note.
# Invoked by `android-acceptance.sh --seed`; can also be run manually with BASE_URL /
# PG_CONTAINER / DB_NAME set.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

base_url=${BASE_URL:-http://127.0.0.1:18080/api/v1}
pg_container=${PG_CONTAINER:-}
db_name=${DB_NAME:-hdd_accept}

if [ -z "$pg_container" ]; then
  echo "seed-acceptance: PG_CONTAINER is required" >&2
  exit 1
fi
for cmd in curl python3 docker; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "seed-acceptance: $cmd is required" >&2
    exit 1
  fi
done

json_field() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"
}

report_id_of() {
  python3 -c 'import json,sys; print(json.load(sys.stdin)["report"]["report_id"])'
}

today=$(python3 -c 'from datetime import date; print(date.today())')
growth_old=$(python3 -c 'from datetime import date,timedelta; print(date.today()-timedelta(days=120))')
latest_report=$(python3 -c 'from datetime import date,timedelta; print(date.today()-timedelta(days=30))')
older_report=$(python3 -c 'from datetime import date,timedelta; print(date.today()-timedelta(days=120))')
note_date=$(python3 -c 'from datetime import date,timedelta; print(date.today()-timedelta(days=45))')

member=$(curl -fsS -X POST "$base_url/members" -H 'Content-Type: application/json' \
  -d '{"name":"小虎","sex":"male","birth_date":"2018-05-20"}')
member_id=$(printf '%s' "$member" | json_field member_id)
echo "seed-acceptance: member_id=$member_id"

curl -fsS -X POST "$base_url/members/$member_id/growth" -H 'Content-Type: application/json' \
  -d "{\"measured_on\":\"$today\",\"height_cm\":128.5,\"weight_kg\":27.4,\"source\":\"measured\"}" >/dev/null
curl -fsS -X POST "$base_url/members/$member_id/growth" -H 'Content-Type: application/json' \
  -d "{\"measured_on\":\"$growth_old\",\"height_cm\":127,\"source\":\"report\"}" >/dev/null

upload_report() {
  local file=$1
  curl -fsS -X POST "$base_url/reports" -F "image=@samples/$file" -F "member_id=$member_id" \
    | report_id_of
}

liver_id=$(upload_report lft_02.jpg)
older_id=$(upload_report lft_04.jpeg)
note_id=$(upload_report lft_05.jpeg)

docker exec -i "$pg_container" psql -U postgres -d "$db_name" -v ON_ERROR_STOP=1 <<SQL
UPDATE reports SET report_date='$latest_report' WHERE image_sha256='$liver_id';
UPDATE reports SET report_date='$older_report' WHERE image_sha256='$older_id';
DELETE FROM report_items WHERE report_sha256='$note_id';
UPDATE reports SET report_type='clinical_note', panels='{clinical_note}', report_date='$note_date', status='success',
  sections='[{"kind":"diagnosis","text":"考虑缺铁性贫血","ocr_line_numbers":[0]},{"kind":"treatment","text":"补铁治疗，一个月后复查。","ocr_line_numbers":[1]}]'::jsonb
WHERE image_sha256='$note_id';
SQL

cat <<EOF
seed-acceptance: ready
  member_id:    $member_id
  reports:      $liver_id (latest), $older_id (older), $note_id (clinical_note)
  growth:       measured $today / report $growth_old
  App Base URL: $base_url
EOF
