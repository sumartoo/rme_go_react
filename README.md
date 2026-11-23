# JWT Auth: Go + React + PostgreSQL

Register and log in with JWT. A protected page shows `Hello [email], welcome back`. You are logged out after 15 minutes of inactivity.

| Layer    | Stack                                                        |
|----------|--------------------------------------------------------------|
| Backend  | Go 1.26, `net/http`, `pgx`, `golang-jwt/jwt/v5`, bcrypt      |
| Frontend | React 18 + TypeScript (Vite), React Router                   |
| Database | PostgreSQL 16                                                |
| Runtime  | Docker Compose (db, backend, frontend served by nginx)       |

## Quick start (Docker)

Requirements: Docker with Docker Compose v2.

```bash
cp .env.example .env          # then set JWT_SECRET to a random string of 32+ characters
docker compose up --build
```

- Frontend: http://localhost:3000
- Backend API: http://localhost:8080 (the frontend also proxies `/api` to it)
- PostgreSQL: localhost:5432

On startup the backend applies the SQL files in `backend/internal/db/migrations` that have not run yet (tracked in the `schema_migrations` table), creating the `users`, `menus`, `roles` and `role_menus` tables. To stop and remove the data: `docker compose down -v`.

## Environment variables

| Variable            | Required | Default   | Description                                                     |
|---------------------|----------|-----------|-----------------------------------------------------------------|
| `JWT_SECRET`        | yes      | none      | HS256 signing secret, at least 32 characters (`openssl rand -hex 32`) |
| `JWT_TTL`           | no       | `15m`     | Token lifetime and inactivity timeout (Go duration: `15m`, `1m`) |
| `POSTGRES_USER`     | no       | `app`     | Database user                                                    |
| `POSTGRES_PASSWORD` | no       | `app`     | Database password                                                |
| `POSTGRES_DB`       | no       | `authdb`  | Database name                                                    |
| `FRONTEND_PORT`     | no       | `3000`    | Host port for the web app                                        |
| `BACKEND_PORT`      | no       | `8080`    | Host port for the API                                            |
| `DB_PORT`           | no       | `5432`    | Host port for PostgreSQL                                         |
| `SWAGGER_ENABLED`   | no       | `false`   | Serve Swagger UI at `/swagger/`. Keep `false` in production      |
| `SEED_DEMO_USERS`   | no       | `false`   | Create the [demo users](#demo-users) on startup. Keep `false` in production |

When running the backend outside Docker it reads `DATABASE_URL` (e.g. `postgres://app:app@localhost:5432/authdb?sslmode=disable`), `JWT_SECRET`, `JWT_TTL` and `PORT` (default `8080`). Compose builds `DATABASE_URL` from the `POSTGRES_*` values.

## API

| Method | Path                 | Auth   | Body                                   | Response |
|--------|----------------------|--------|----------------------------------------|----------|
| POST   | `/api/auth/register` | none   | `{email, password, confirmPassword}`   | `201 {id, email}`; `400 {errors}`; `409` if the email is taken |
| POST   | `/api/auth/login`    | none   | `{email, password}`                    | `200 {token, expiresAt}`; `401` |
| POST   | `/api/auth/refresh`  | Bearer | none                                   | `200 {token, expiresAt}` (new token) |
| GET    | `/api/me`            | Bearer | none                                   | `200 {id, email, message: "Hello <email>, welcome back"}` |
| GET    | `/api/me/branches`   | Bearer | none                                   | `200 {branches: [{kode, nama}]}`: the branches the caller is mapped to (`refbranchauth`), shown on the home page; no menu needed |
| GET    | `/api/menus`         | Bearer | none                                   | `200 {menus: [{id, title, icon, path, children}], role}`: the sidebar menu for the caller's role |
| GET    | `/api/roles`         | Bearer + Role menu | none                       | `200 {roles: [{id, name, allAccess, menuIds, userCount}], menus}` |
| POST   | `/api/roles`         | Bearer + Role menu | `{name, allAccess, menuIds}` | `201` role; `400`; `409` if the name exists |
| PUT    | `/api/roles/{id}`    | Bearer + Role menu | `{name, allAccess, menuIds}` | `200` role; `400` if it would remove your own access to the Role page |
| DELETE | `/api/roles/{id}`    | Bearer + Role menu | none                       | `204`; `409` while users still have the role |
| GET    | `/api/users`         | Bearer + User menu | none                       | `200 {users: [{id, email, roleId}], roles: [{id, name}]}` |
| PUT    | `/api/users/{id}/role` | Bearer + User menu | `{roleId}` (null = no role) | `200`; `400` for your own account |
| PUT    | `/api/users/{id}/branches` | Bearer + User menu | `{branches: ["MED", "PRM"]}` | `200`; `400` for an unknown branch |
| GET    | `/api/users/{id}/menus` | Bearer + User menu | none                    | `200 {role, allAccess, menus}`: the menu tree that user's sidebar shows |
| GET    | `/api/branches`      | Bearer + Cabang menu | none                     | `200 {branches: [...]}` (secret key masked) |
| POST   | `/api/branches`      | Bearer + Cabang menu | branch fields            | `201` branch; `400`; `409` if the code is used |
| PUT    | `/api/branches/{kode}` | Bearer + Cabang menu | branch fields (kode ignored; empty `secretKey` keeps it) | `200` branch; `404` |
| GET    | `/healthz`           | none   | none                                   | `ok` |

Validation rules: the email must be valid and is stored lowercase; the password must be 8–72 characters; `confirmPassword` must equal `password`. Validation errors come back per field:

```json
{ "error": "validation failed", "errors": { "confirmPassword": "Passwords do not match" } }
```

JWT claims: `sub` (user id, UUID), `email`, `iat`, `exp`.

### Swagger / OpenAPI

With `SWAGGER_ENABLED=true` (set in `.env.example`), the backend serves:

- Swagger UI: http://localhost:8080/swagger/
- OpenAPI 3 spec: http://localhost:8080/swagger/openapi.yaml

To try a protected endpoint, call `POST /api/auth/login`, click **Authorize** and paste the `token`.

The variable defaults to `false`, so in production the docs are hidden (both URLs return `404`) unless you turn them on. The spec is written by hand in [backend/internal/docs/openapi.yaml](backend/internal/docs/openapi.yaml) and embedded in the binary, so update it whenever an endpoint changes. A test fails if a route is missing from it. The UI page loads Swagger UI's assets from cdn.jsdelivr.net.

Example:

```bash
curl -X POST localhost:8080/api/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password1","confirmPassword":"password1"}'

TOKEN=$(curl -s -X POST localhost:8080/api/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"password1"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')

curl localhost:8080/api/me -H "Authorization: Bearer $TOKEN"
# {"email":"alice@example.com","id":"...","message":"Hello alice@example.com, welcome back"}
```

## Sidebar menu (configured in the database)

After login every page shares a layout with a sidebar. The sidebar is built from the `menus` table, so you change it with SQL (or DBeaver) and users see the change after reloading the page. No restart or redeploy is needed.

| Column       | Meaning |
|--------------|---------|
| `parent_id`  | `NULL` for a top-level item, otherwise the `id` of its group. Any depth works |
| `title`      | Text shown in the menu |
| `icon`       | Icon name: `home`, `database`, `circle`, `users`, `user`, `settings`, `heart`, `chart`, `bar-chart`, `monitor`, `boxes`, `upload`, `barcode`, `dollar`, `file`, `user-circle`, `copy`, `building`. Unknown names show a dot. Add names in [frontend/src/layout/icons.tsx](frontend/src/layout/icons.tsx) |
| `path`       | Frontend route the item opens. Leave `NULL` on a group: clicking a group opens or closes its children |
| `sort_order` | Lower comes first among siblings |
| `is_active`  | `false` hides the item and everything under it |

```sql
-- Add a child under "Setting" (it becomes a group)
INSERT INTO menus (parent_id, title, icon, path, sort_order)
SELECT id, 'User Management', 'users', '/setting/users', 10 FROM menus WHERE title = 'Setting' AND parent_id IS NULL;
UPDATE menus SET path = NULL WHERE title = 'Setting' AND parent_id IS NULL;

-- Hide an item
UPDATE menus SET is_active = false WHERE title = 'Upload';
```

The first menu is seeded by `002_create_menus.sql` only once, so later edits and deletes are kept across restarts. A route without its own React page shows a placeholder with the menu title. To build a real page, add a `<Route>` for its path inside the layout route in [frontend/src/main.tsx](frontend/src/main.tsx), wrapped in `<MenuGuard>` so only roles that have the menu can open it.

## Roles and menu access

Each user has one role, and the role decides which menus they see. Admins manage this in the app, with no SQL needed:

- **Management › Role**: create, rename or delete roles, and tick the menus each role can open. Ticking a group grants every item in it, including items added later. **All menus** grants everything, including menus created later.
- **Management › User**: pick a role for each user, and with the building icon the branches (cabang) they work at. A user can belong to several branches. The **Action** icon shows, as a tree, exactly which menus that user gets from their role. A user without a role (for example someone who just registered) sees an empty sidebar until an admin assigns one.

Changes apply on the user's next page load, without a restart or a new login, because access is read from the database on every request.

| Table / column       | Meaning |
|----------------------|---------|
| `roles`              | `name` (unique) and `all_access` (every menu, including future ones) |
| `role_menus`         | `(role_id, menu_id)` pairs. A group id grants everything under it, and the groups above a granted item are shown so it can be reached |
| `users.role_id`      | The user's role, or `NULL` for no menus. A role that still has users cannot be deleted |
| `refbranchauth`      | User-to-branch mapping, one row per pair: `branch` (refbranch.kode) and `userid` (users.id). Same columns as `refbranchauth` in `lab`, plus a unique `(branch, userid)` and a foreign key to `refbranch`. Branches do not grant page access; they are there for pages that filter data by branch |

Access is also enforced on the server. The role and user APIs check that the caller's role can open `/management/role` or `/management/user`. The rule is "whoever has the menu can use its page", so giving a role the Role menu effectively makes it an admin role. To stop admins from locking themselves out, you cannot remove the Role page from your own role or change your own role.

Seeded by `017_roles.sql`:

| Role          | Menus |
|---------------|-------|
| `Admin`       | All menus |
| `Pendaftaran` | Home, Master Data Klinik, Front Office Klinik, Report Klinik |
| `Perawat`     | Home, Transaksi Klinik, Report Klinik |

### Demo users

With `SEED_DEMO_USERS=true` (set in `.env.example`), the backend creates these users on startup if they do not exist yet. It never overwrites an existing user.

| Email                | Password    | Role          |
|----------------------|-------------|---------------|
| `admin@medika.com`   | `Medika123` | `Admin`       |
| `cs@medika.com`      | `Medika123` | `Pendaftaran` |
| `perawat@medika.com` | `Medika123` | `Perawat`     |

The password is public, so keep `SEED_DEMO_USERS=false` in production. If the demo users were already created somewhere they should not exist, delete them.

## Management › Cabang (branches)

`/management/cabang` lists clinic branches with their Satu Sehat settings. **+ Add** and the edit icon open a form. Use Status `Nonaktif` to retire a branch; there is no delete.

The data lives in the `refbranch` table, which has the same columns, types and primary key (`pk_refbranch` on `kode`) as `refbranch` in the existing `lab` database. Migration `018_branches.sql` creates the table and `019_refbranch.sql` reshapes it.

- `kode` is the primary key, so it is set on create and cannot be changed afterwards (`PUT /api/branches/{kode}`).
- `isactive` and `isinternal` are `1`/`0` in the database and booleans in the API. Empty optional fields are stored as `NULL`, like the lab data.
- Field limits in the form and API match the `varchar` sizes (for example `nama` 50, `kode_kwt` 10).
- The table shows the columns of the original screen. Kode KWT, Alamat, Telp, NPWP, Nama NPWP and Internal start hidden; turn them on under **Column visibility**.
- **Secret Key is write-only.** The API never returns it, only whether it is set and its last 4 characters. When editing, leave it blank to keep the current key. The key is stored as plain text in the database because the server needs it to call Satu Sehat, so limit who can read the database.
- The 11 seeded branches match lab for the non-secret columns. Alamat, Telp, NPWP and the keys are not seeded; enter them in the form.

The table is the reusable [DataTable](frontend/src/components/DataTable.tsx) component, ready for other list pages. It provides:

- records per page, search, column sorting, column visibility and paging;
- **Copy** (tab-separated, for pasting into a spreadsheet), **Excel** (a real `.xlsx`), **CSV**, **PDF** and **Print**.

Exports contain every row that matches the search, in the current sort order, with the visible columns only. The PDF library (`jspdf`) loads only when PDF is clicked.

## Master Data Klinik › Obat (drugs and stock per branch)

`/master-data-klinik/obat` lists the drug master and each drug's stock and prices **per branch**.

| Table          | Content |
|----------------|---------|
| `refobat`      | Drug master: name, jenis, units, default prices, distributor, principal, komposisi, … |
| `trxstock`     | One row per drug and branch (`id_obat`, `cabang`): `stok`, `stok_min`, `hbeli`, `hp`, `hjual`, `isactive` |
| lookups        | `refjenisobat`, `refsatuan`, `reflogoobat`, `refprodusen`, `refkodemovingobat`, `refkodemovingpo`, `refdistributorobat`, `refprincipalobat`, `refgolonganobat` |

These tables have the same columns, types and primary keys as in the `lab` database (migration `021_obat.sql`). The app adds foreign keys where the lab data is consistent. `refobat.id_prod` has none because most values point to producers that no longer exist.

**Branch authorization.** Users see and edit stock only for the branches they are mapped to under Management › User (`refbranchauth`):

- `GET /api/obat` returns every drug, but `stocks` only for the caller's branches.
- `PUT /api/obat/{id}/stock/{cabang}` returns `403` for any other branch.
- Several branches: the page shows them all, one line per branch, and the **Cabang** dropdown narrows to one. One branch: plain values. No branch: stock columns are empty and the **$** button is disabled.
- The mapping is read on every request, so changes apply immediately.

Who can open the page at all is decided by the role (the Obat menu item).

**Editing.**

- The pencil edits the drug master; `create_user`/`update_user` get the user's email cut to 20 characters (the column size).
- The **$** button edits stock and prices for one of the user's branches. Saving creates the `trxstock` row if the drug has none at that branch yet.
- Upload (import from a file) is not built yet.

### Importing everything from lab

`scripts/import-lab-all.sh` runs all the `import-lab-*.sh` scripts below in dependency order and stops at the first failure (a few minutes in total):

```bash
LAB_PASSWORD=... scripts/import-lab-all.sh            # into an empty app database
LAB_PASSWORD=... scripts/import-lab-all.sh --replace  # wipe and copy everything again, e.g. to pull new lab data
```

`--replace` also deletes rows entered in the app since the last import. The scripts write to `DATABASE_URL` from `.env` when it is set (the same database the backend uses), otherwise to the `db` container.

### Importing drugs from lab

The drug data is not in the migrations. Copy it from the lab database (with the stack running):

```bash
LAB_PASSWORD=... scripts/import-lab-obat.sh            # only when the app has no drugs yet
LAB_PASSWORD=... scripts/import-lab-obat.sh --replace  # wipe the app's drug tables and copy again
```

`LAB_HOST` (default `host.docker.internal`, the Docker host), `LAB_PORT`, `LAB_DB` and `LAB_USER` can be overridden. Stock rows for drugs deleted from lab (about 4,200) are skipped. The import takes about 10 seconds for 2,485 drugs and 24,570 stock rows. Re-run it after `docker compose down -v`, which deletes the database.

## Master Data Klinik › Tindakan (procedures and prices per branch)

`/master-data-klinik/tindakan` works like the Obat page.

| Table               | Content |
|---------------------|---------|
| `reftindakan`       | Procedure: name, jenis, default prices (`hbeli`, `hjual`, `hjual_jamkes`), `isactive`, `isedit`, `sts_ctk_krtu` |
| `reftindakandetail` | One row per procedure and branch: `hbeli`, `hjual`, `hjual_jamkes` (price for health-insurance patients) and `isactive` |
| `refjenistindakan`  | Procedure types |

The tables have the same layout as in `lab` (migration `022_tindakan.sql`). The app adds:

- a unique `(id_tindakan, cabang)`, so there is one price row per procedure and branch;
- foreign keys to `reftindakan` and `refbranch`.

There is no foreign key on `id_jenis`, because one procedure in lab points to a type that no longer exists.

Prices follow the same branch authorization as Obat:

- `GET /api/tindakan` returns `prices` only for the caller's branches.
- `PUT /api/tindakan/{id}/price/{cabang}` returns `403` for any other branch.
- The pencil edits the procedure itself; the **$** button edits the prices at one of the user's branches.

Import the data from lab with:

```bash
LAB_PASSWORD=... scripts/import-lab-tindakan.sh            # only when the app has no procedures yet
LAB_PASSWORD=... scripts/import-lab-tindakan.sh --replace  # wipe and copy again
```

The three duplicate price rows in lab (same procedure and branch, same values) are imported once. Shared import helpers live in `scripts/lib/lab.sh`.

## Master Data Klinik › Penunjang (supporting examinations and prices per branch)

`/master-data-klinik/penunjang` works like Tindakan, for lab, radiology and other supporting examinations.

| Table                | Content |
|----------------------|---------|
| `refpenunjang`       | Examination: name, type, default `hjual` and `hjual_jamkes`, `isactive` |
| `refpenunjangdetail` | One row per examination and branch: `hjual`, `hjual_jamkes`, `isactive` |
| `refjenispanjang`    | Examination types (LABORATORIUM, RADIOLOGI, LAB MOHC, …). The table is named like this in `lab`, and the name is kept |

- **No purchase price:** these tables in `lab` have no `hbeli` column, so there is none here either.
- **Same layout as `lab`** (migration `023_penunjang.sql`), plus a unique `(id_penunjang, cabang)` and foreign keys to the type, examination and branch. The lab data satisfies all of them.
- **Branch authorization** is the same as Obat and Tindakan (`PUT /api/penunjang/{id}/price/{cabang}` returns `403` outside the caller's branches).

```bash
LAB_PASSWORD=... scripts/import-lab-penunjang.sh            # only when the app has no examinations yet
LAB_PASSWORD=... scripts/import-lab-penunjang.sh --replace  # wipe and copy again
```

## Master Data Klinik › Pasien (patients, paged on the server)

`/master-data-klinik/pasien` lists the ~430,000 patients of `refmpasien`. The table runs in server-side mode (`DataTable`'s `remote` option): paging, search (name, No RM, NIK, address, or Kode) and sorting happen in PostgreSQL, so each page loads in well under a second. Exports fetch at most the first 5,000 matching rows.

| Table | Content |
|-------|---------|
| `refmpasien` | Patients |
| `refgender`, `refgoldarah`, `refagama`, `refstatuskawin` | Gender, blood type, religion, marital status |
| `refjaminankesehatan` | Health insurers |
| `refkelurahan` › `refkecamatan` › `refkabupaten` › `refprovinsi` | Region hierarchy. Patients point to `refkelurahan`; the form fills kecamatan, kab/kota and provinsi when a kelurahan is picked |

The tables have the same columns as in `lab`. The app adds the following:

- **Migration `024_pasien.sql`**: search and sort indexes, including trigram (`pg_trgm`) indexes.
- **Migration `025_pasien_keys.sql`**:
  - primary keys on the lookups that lack one in `lab` (gender, agama, status kawin, kelurahan);
  - foreign keys for every reference, which the lab data satisfies.

**Branch authorization.**

- Users see, open and edit only patients whose `cabang` is one of their branches (`refbranchauth`). A patient of another branch answers `404`.
- New patients and moved patients must go to one of the user's branches.
- The 19 lab patients without a branch are not listed for anyone.

**No RM** is generated on create and never changes:

- It is the branch's newest number + 1 in the `NN-NN-NN` format (e.g. `89-01-17` → `89-01-18`), skipping numbers already used in that branch.
- Generation is serialized per branch with an advisory lock.
- No RM is not unique in `lab`; numbers repeat across branches, and 96 even repeat within a branch. So it has no unique constraint.

**Print card** opens a printable card with the name, a Code 128 barcode of the No RM ([barcode.ts](frontend/src/components/barcode.ts)) and the No RM.

**Data notes (lab):**

- 175 birth dates are impossible (years 1000 to 9999). They show no age, sort last, and must be corrected before the patient can be saved.
- Empty branch codes were imported as `NULL`.

### Importing patients from lab

```bash
LAB_PASSWORD=... scripts/import-lab-pasien.sh            # only when the app has no patients yet
LAB_PASSWORD=... scripts/import-lab-pasien.sh --replace  # wipe and copy again (about 40 seconds)
```

This copies real personal and medical data (names, NIK, addresses, allergies). Keep it only on machines allowed to hold it, never commit a dump, and consider anonymizing it for shared development databases.

## Master Data Klinik › Dokter (doctors, their procedures and practice branches)

`/master-data-klinik/dokter` has three tabs:

- **Data Dokter**: the 689 doctors of `refmdokter`, with type, gender, status (`status` 1 = Active, 2 = Non Active) and practice branches. **+ Add** and the pencil open the form.
- **Tindakan Dokter**: the procedures a doctor may perform (`trxtindakan` → `reftindakan`). The left list holds the doctor's procedures; remove them with the trash icon, or tick several and press **Delete**. The right list holds the other procedures; tick them and press **Move**.
- **Cabang Praktek**: the branches where the doctor practises (`refmdokterauth`), with the same two lists. Users can only add or remove **their own branches** (`refbranchauth`); other branches show a lock, and the API answers `403` for them.

Open the second and third tab for a doctor with the row buttons in Data Dokter.

| Table | Content |
|-------|---------|
| `refmdokter` | Doctors. Legacy fields `id_internal`, `cabang`, `oldid` and `ihs_*` are kept but not edited |
| `refjenisdokter` | Doctor types (DOKTER UMUM, DOKTER GIGI, …) |
| `refmdokterauth` | Doctor ↔ practice branch |
| `trxtindakan` | Doctor ↔ procedure |

The tables have the same layout as in `lab`. Migration `026_dokter.sql` creates them; `027_dokter_keys.sql` adds:

- foreign keys to the lookups (shared with the patient tables) and to `reftindakan` and `refbranch`;
- unique `(dokterid, cabang)` and `(id_dokter, id_tindakan)`.

Lab has 1,512 duplicate doctor/procedure pairs; the import loads each pair once.

```bash
LAB_PASSWORD=... scripts/import-lab-dokter.sh            # needs the Pasien and Tindakan imports first
LAB_PASSWORD=... scripts/import-lab-dokter.sh --replace
```

## Master Data Klinik › Jaminan Kesehatan (health insurers)

`/master-data-klinik/jaminan-kesehatan` lists the insurers of `refjaminankesehatan`. The columns are type, status (`isactive` 1 = Active, 2 = Non Active) and limit (`status_limit` 1 = Limit with the `limit` amount, 2 = No Limit). Each row has three actions:

- **Pencil**: edit the insurer.
- **Ketentuan PKS**: the agreement terms of `refjaminankesehatan_detail`, one text per term of `refjaminankesehatan_pksmaster`: Masa berlaku PKS, Tarif, Biaya Adm, Biaya Konsul, Batas Tagihan Kadaluarsa and Batas Pembayaran kadaluarsa. Leave a term empty to remove it.
- **Detail item (i)**: the insurer's numbered list in `refjaminankesehatan_detail_item` (in lab, exclusions such as "Penyakit akibat alkohol dan narkoba"). Items can be added, reordered and removed; they are renumbered 1, 2, 3, … on save.

Migration `028_jamkes.sql`:

- creates the type table (`refjaminankesehatantype`: PT, CV, Non PT CV, Payor Admedika, Non Payor Admedika, PT & CV), the PKS term list and the two detail tables, with the same columns as `lab`;
- seeds the 6 types and 6 terms;
- adds a primary key on the term list, foreign keys, and a unique `(jamkesid, detailid)` for the terms.

The insurers come with the patient import. The details have their own import:

```bash
LAB_PASSWORD=... scripts/import-lab-jamkes.sh            # PKS terms and item lists
LAB_PASSWORD=... scripts/import-lab-jamkes.sh --replace
```

Re-running `import-lab-pasien.sh --replace` empties the insurer details (they reference the insurers), so run `import-lab-jamkes.sh` again afterwards.

## Master Data Klinik › Distributor

`/master-data-klinik/distributor` lists the drug distributors of `refdistributorobat`, with these columns: Kode, Nama, Alamat, Telp, Fax, Kontak Person (`cp`) and Telp KP (`telp_cp`). **+ Add** and the pencil open the form. Field limits follow the `varchar` sizes (Nama 300, Alamat 200, Telp 15, Fax 25, Kontak Person 40, Telp KP 25).

The table and its data come with the Obat migration and import (`021_obat.sql`, `scripts/import-lab-obat.sh`), so this page needs no migration or import of its own. There is no delete, because every drug refers to its distributor.

## Master Data Klinik › Principal

`/master-data-klinik/principal` lists the drug principals of `refprincipalobat` (Kode, Nama). **+ Add** and the pencil open a one-field form (Nama, at most 300 characters).

The table and its data come with the Obat migration and import (`021_obat.sql`, `scripts/import-lab-obat.sh`), so this page needs no migration or import of its own. There is no delete, because drugs refer to their principal.

## Master Data Klinik › ICD

`/master-data-klinik/icd` lists the 18,505 ICD-X diagnosis codes of `reficd` (Kode, Kode ICD-X, Nama Penyakit, Jenis ICD-X).

- **Server-side mode.** Like Pasien, paging, search (code, disease name or type) and sorting run in the database, and exports fetch at most 5,000 matching rows.
- **Form.** **+ Add** and the pencil open the form; Kode ICD-X is stored uppercase and must be unique (`409` otherwise).

Migration `029_icd.sql` creates `refjenisicd` (GIGI, THT, SARAF, …) and `reficd` with the lab layout. It adds a unique `kode_icd`, a foreign key to the type, and trigram indexes for search; the lab data satisfies all of them.

```bash
LAB_PASSWORD=... scripts/import-lab-icd.sh            # only when the app has no ICD codes yet
LAB_PASSWORD=... scripts/import-lab-icd.sh --replace
```

## Master Data Klinik › Penjual OTC (Perujuk OTC)

`/master-data-klinik/penjual-otc` lists the people who refer over-the-counter sales (`refperujukotc`: ID, Nama), titled "Perujuk OTC" like the original screen. **+ Add** and the pencil open a one-field form (Nama, at most 50 characters). There is no delete, because sales transactions (`trxjual.id_perujuk_otc` in lab) refer to these rows. Names are not unique in lab (one appears twice), so there is no unique constraint.

```bash
LAB_PASSWORD=... scripts/import-lab-perujuk-otc.sh            # migration 030_perujuk_otc.sql creates the table
LAB_PASSWORD=... scripts/import-lab-perujuk-otc.sh --replace
```

## Master Data › Pasien (MCU patients)

`/master-data/pasien` lists the MCU patients (`refpasien`, ~7.3k rows), separate from the clinic patients of Master Data Klinik › Pasien (`refmpasien`). Columns: No RM, Nama, Jenis Kelamin, Tanggal Lahir, No Telp, No Hp, Alamat, Perusahaan (name from `refcompany`), NIK, Email, Departmen, Posisi. The list is paged, searched (No RM, name, NIK, address, company) and sorted in the database, by No RM by default. Soft-deleted rows (`isdeleted`) are hidden.

**+ Add** and the pencil open a form; Nama, Jenis Kelamin (`L`/`P`) and Perusahaan are required. A new patient gets the next No RM of today, `YYMMDD` (Asia/Jakarta) plus a 4-digit sequence, like lab; No RM is not editable. No RM is not unique in lab (13 numbers appear twice), so there is no unique constraint. There is no delete, and the original screen's **Upload** button is not built yet.

```bash
LAB_PASSWORD=... scripts/import-lab-pasien-mcu.sh            # migration 042_pasien_mcu.sql creates the table
LAB_PASSWORD=... scripts/import-lab-pasien-mcu.sh --replace
```

## Master Data › Perusahaan

`/master-data/perusahaan` lists the client companies (`refcompany`: Kode Perusahaan, Nama Perusahaan, Alamat), sorted by code like the original screen. **+ Add** and the pencil open a form with Kode Perusahaan (required, at most 10 characters, unique), Nama Perusahaan (required, 255), Alamat (255), Contact Person (100), Email (100), Telepon (30), Fax (30) and PPN (`isvat`: Ya = `1`, Tidak = `0`, or empty, as most rows in lab are). There is no delete. The code is unique in lab, so the app enforces it (`409` with a field error when taken).

```bash
LAB_PASSWORD=... scripts/import-lab-perusahaan.sh            # migration 041_perusahaan.sql creates the table
LAB_PASSWORD=... scripts/import-lab-perusahaan.sh --replace
```

## Master Data › Lab Master

`/master-data/lab-master` lists the groups of lab examinations (`reflabmaster`: Lab Master Code, Lab Master Name), sorted by code like the original screen. **+ Add** and the pencil open a form with Lab Master Code (at most 20 characters) and Lab Master Name (at most 50); saves record `createby`/`modifiedby` and the time. There is no delete, and soft-deleted rows (`isdeleted`) are hidden. lab has no unique constraint on the code, so neither does the app.

```bash
LAB_PASSWORD=... scripts/import-lab-lab-master.sh            # migration 043_lab_master.sql creates the table
LAB_PASSWORD=... scripts/import-lab-lab-master.sh --replace
```

## Master Data › Dokter (MCU doctors)

`/master-data/dokter` lists the MCU doctors (`refdoctor`: Dokter Code, Dokter Name, Dokter Master Name from `refdoctormaster`, Percentage Fee), sorted by code like the original screen. They are separate from the clinic doctors of Master Data Klinik › Dokter (`refdokter`). **+ Add** and the pencil open a form with Dokter Code (required on add, at most 20 characters, unique; it is the primary key, so it cannot be changed later), Dokter Name (required, 50), Dokter Master (required, a list) and Percentage Fee (optional, 0–100). There is no delete.

```bash
LAB_PASSWORD=... scripts/import-lab-dokter-mcu.sh            # migration 048_dokter_mcu.sql creates the table
LAB_PASSWORD=... scripts/import-lab-dokter-mcu.sh --replace
```

## Master Data › Dokter Master

`/master-data/dokter-master` lists the kinds of MCU doctor (`refdoctormaster`: Dokter Master Name), sorted by name like the original screen. They are separate from the clinic doctors (`refdokter`). **+ Add** and the pencil open a one-field form (Dokter Master Name, at most 50 characters). There is no delete.

```bash
LAB_PASSWORD=... scripts/import-lab-dokter-master.sh            # migration 047_dokter_master.sql creates the table
LAB_PASSWORD=... scripts/import-lab-dokter-master.sh --replace
```

## Master Data › Non Lab

`/master-data/non-lab` lists the services sold with an MCU that are not lab examinations (`refnonlab`: ID, Non Lab Name, Price), ordered by ID with 20 rows per page like the original screen. **+ Add** and the pencil open a form with Non Lab Name (required, at most 100 characters) and Price (optional, from 0); saves record `createby`/`modifiedby` and the time. There is no delete, and soft-deleted rows (`isdeleted`) are hidden.

```bash
LAB_PASSWORD=... scripts/import-lab-non-lab.sh            # migration 046_non_lab.sql creates the table
LAB_PASSWORD=... scripts/import-lab-non-lab.sh --replace
```

## Master Data › Lab

`/master-data/lab` lists the lab examinations (`reflab`, 593 rows): Lab Code (`labid`), Lab Master, Sub Master Name (from `reflabsubmaster` and `reflabmaster`), Lab (`labname`) and Status, ordered by Lab Code like the original screen. Here `isdeleted` is the Status: `true` = Non Active (183 rows in lab); those rows stay listed. **+ Add** and the pencil open a form with Sub Master (grouped by Lab Master), Lab (at most 50 characters), Kode (`labcode`, at most 50; empty = same as Lab, as in most of lab), Price, Fee (0–100; 15 for a new examination, like lab's default) and Status. Deactivating records `deletedby`/`deleteddate`, activating clears them; saves also record `createby`/`modifiedby`. There is no delete.

```bash
LAB_PASSWORD=... scripts/import-lab-lab.sh            # migration 045_lab.sql creates the table
LAB_PASSWORD=... scripts/import-lab-lab.sh --replace
```

## Master Data › Lab Sub Master

`/master-data/lab-sub-master` lists the subgroups of lab examinations (`reflabsubmaster`: Lab Master, Sub Master Code, Sub Master Name), ordered by Lab Master and then entry order like the original screen. The Lab Master column shows the name from `reflabmaster`. **+ Add** and the pencil open a form with Lab Master (a list of the Lab Masters), Sub Master Code (at most 20 characters) and Sub Master Name (at most 50); saves record `createby`/`modifiedby` and the time. There is no delete, and soft-deleted rows (`isdeleted`) are hidden.

```bash
LAB_PASSWORD=... scripts/import-lab-lab-sub-master.sh            # migration 044_lab_sub_master.sql creates the table
LAB_PASSWORD=... scripts/import-lab-lab-sub-master.sh --replace
```

## Master Data › MCU

`/master-data/mcu` lists the MCU examination groups (`refmastermcu`: ID, Kode Mcu, Nama Mcu), like the original screen. **+ Add** and the pencil open a form with Kode Mcu (at most 10 characters) and Nama Mcu (at most 30). There is no delete. lab has no unique constraint on the code, so neither does the app.

```bash
LAB_PASSWORD=... scripts/import-lab-master-mcu.sh            # migration 040_master_mcu.sql creates the table
LAB_PASSWORD=... scripts/import-lab-master-mcu.sh --replace
```

## Front Office Klinik › Pendaftaran Pasien (patient registrations)

`/front-office-klinik/pendaftaran-pasien` lists registrations (`trxpendaftaran`, 344k rows in lab) with server-side paging, search and sorting, like Pasien. The columns are No Pendaftaran, No Antrian, No Antrian Poli, Tanggal Waktu, Pelayanan, Status Kunjungan, No RM, Nama, Tanggal Lahir, Umur, Alamat, Cabang, Pakai Kartu, Total Point and Action.

- **Filters.** A date range (both ends default to today), a branch selector, and a **Filter** popover with Pelayanan and Status. Exports fetch at most 5,000 matching rows.
- **Pelayanan** comes from `refjenisrawat` (POLI UMUM, POLI GIGI, LABORATORIUM, OBAT BEBAS, …). `refjenispanjang` holds the supporting examination types used by Penunjang, not the registration services.
- **Status Kunjungan** comes from `refstatustransaksi`: NON KUN, CLOSE, DAFTAR, PROSES, PAYMENT, CANCEL. A new registration starts as DAFTAR. It can be edited (Pelayanan, Pakai Kartu) or cancelled only while it is still DAFTAR. Cancelling records who cancelled it, when, and the reason, which shows as a tooltip on the status.
- **Total Point** is the patient's point balance in `refpasienpoint`.
- **+ Add** opens a dialog with Tanggal (today or earlier), Pelayanan, Cabang and Pakai Kartu, above a patient search table where you tick one patient. Cabang is filled from the patient's branch and can be changed. **Cari** opens the legacy "Pencarian" panel: pick a field (No RM, Nama, Umur, Alamat, Jam Kes), type a key and press the search button or Enter. Umur takes whole years and matches patients exactly that old today.
- **Queue numbers** follow lab:
  - No Antrian counts per branch per day.
  - No Antrian Poli counts per branch, day and service. It gets a printed form only for services with `neednoantrian = 1`: the service's `firstcode` plus three digits, for example `U009` or `G001`.
  - Numbering is serialized per branch and day with a PostgreSQL advisory lock. Changing the service of a registration gives it a new poli number.
- **Time zone.** Dates and times are the clinic's (`Asia/Jakarta`), whatever the server's zone is. The time zone database is embedded in the binary, because the Alpine image has none.
- **Branch authorization.** Users only see, add and edit registrations of their own branches (`refbranchauth`). Asking for another branch answers `403`, and another branch's registration answers `404`.

Migration `031_pendaftaran.sql` creates `refjenisrawat`, `refstatustransaksi`, `trxpendaftaran` and `refpasienpoint` with the lab layout. `032_pendaftaran_keys.sql` adds foreign keys to the service, the status and the branch. There is no foreign key to the patient, because 38,174 lab registrations refer to patients that no longer exist; the list shows "(pasien tidak ditemukan)" for them. The import maps 194 empty branch codes to NULL, and copies point balances only for patients that exist in the app. The lab data ends on 2026-11-08, so "today" is empty until new registrations are made.

```bash
LAB_PASSWORD=... scripts/import-lab-pendaftaran.sh            # needs the Pasien import first
LAB_PASSWORD=... scripts/import-lab-pendaftaran.sh --replace
```

The data is personal (who visited which clinic and when): keep it on machines allowed to hold it, and never commit it.

## Transaksi Klinik › Pengobatan (treatment sales)

`/transaksi-klinik/pengobatan` lists treatment sales (`trxjual`, 317k rows in lab) with their totals (`trxjualtotal`), newest first, with server-side paging, search and sorting. The columns are No, Tanggal, No Kuitansi, Pasien, Dokter, Jenis Rawat, Pembayaran, Total, Cabang and Status.

- **Dates.** A date range, both ends defaulting to today (clinic time). A sale counts for a day from 00:00 up to, but not including, 00:00 of the next day.
- **Cari** opens the "Pencarian" panel: Nama Pasien, Jenis Pelayanan (`refjenisrawat`) and Status Kunjungan. Status Kunjungan is the status of the registration the sale belongs to (`trxpendaftaran.id_statustr` via `trxjual.id_kun`). The search button or Enter applies the panel.
- **Filter** has Pembayaran (`refjenisbayar`; the unused duplicate TUNAI and HUTANG rows with flag 2 are hidden) and Status.
- **Status** is the settlement status (`trxjualtotal.status`): 1 Close, 2 Cancel, anything else Open, including the 425 sales without a total row. A cancelled sale shows its reason as a tooltip. Total shows the drugs, procedures, examinations and discount as a tooltip.
- **Search box** matches the receipt number, patient name, No RM, doctor or sale number. Exports fetch at most 5,000 matching rows.
- **Branch authorization.** Users only see the sales of their own branches (`refbranchauth`). Asking for another branch answers `403`.
- **+ Add** opens `/transaksi-klinik/pengobatan/new`, the treatment form laid out like the legacy `PengobatanDetail/New` screen: Cabang, No./Tgl Kuitansi, the Data Pendaftaran and Data Pasien panels, the Obat, Tindakan, Penunjang, Diagnosa and Pemeriksaan tabs, and Data Pembayaran. A sub-page like `…/new` follows its parent menu for access and shows "Add" in the breadcrumb.
- **No. Daftar 🔍** opens the "Data Pendaftaran" lookup (`GET /api/pengobatan/pendaftaran`) and needs a Cabang first. It lists the branch's registrations with status DAFTAR, PROSES or PAYMENT that no sale refers to yet, from yesterday to today by default. It filters by Nama Pasien, Proses (status), Layanan and dates. Picking one fills the registration, the patient and Dokter PJ:
  - Dokter PJ is the examination's doctor (`trxpemeriksaan.id_dok`);
  - a service without an examination gets the branch's NON DOKTER;
  - otherwise it shows "Belum di Pilih".

  Changing the Cabang clears the picked registration.
- **Dokter PJ 🔍** opens the "Data Dokter" lookup (`GET /api/pengobatan/dokter`), which also needs a Cabang. It lists the active doctors (`refmdokter.status` 1) that `refmdokterauth` maps to the branch, by Kode, with Nama Dokter and Jenis (`refjenisdokter`). Changing the Cabang clears Dokter PJ. In lab, CTM has 51 mapped doctors, 36 of them active.
- **Add Obat** (Obat tab) opens the "Data Obat" lookup (`GET /api/pengobatan/obat`): the drugs sold at the branch (`trxstock.isactive` 1) with that branch's price and stock. Type Banyak and press Enter, or tick the row, to add it.
  - Banyak must be more than 0 and may not exceed the branch stock, counting what is already in the list.
  - Picking a drug that is already in the list adds to its quantity.
  - Each line can be edited (Banyak, Dosis) or removed.
  - Changing the Cabang empties the list, because prices and stock are per branch.
- **Totals** follow the legacy rules found in lab (`trxjualobat`, `trxjualtotal`); see `src/pages/pengobatanCalc.ts`:
  - Jumlah = Harga × Banyak;
  - PPN = 10% of Jumlah for drugs with `kena_pajak` 1;
  - "di bulatkan" rounds each line up to a multiple of 500, then sums them. This matches 15,957 of 17,044 recent lab sales; the rest are old unrounded amounts.

  Total Kena Diskon and PPN Kena Diskon equal Total and PPN for now. The drugs' rounded total plus rounded PPN is the bill in Data Pembayaran, and Grand Total, Sisa and Kembalian follow from it.
- **Add Tindakan** (Tindakan tab) needs a Cabang and a Dokter PJ. It opens "Data Tindakan Dokter : <doctor>" (`GET /api/pengobatan/tindakan`), which lists the doctor's procedures (`trxtindakan`) that are active and offered at the branch, with the branch price (`reftindakandetail.hjual`).
  - A procedure with `reftindakan.isedit` 1 has an editable price, in the lookup and in the list.
  - Lines show their doctor (in lab every procedure line carries the sale's doctor), so a new Dokter PJ empties the list, as does a new Cabang.
  - Procedure totals are not rounded: lab stores exactly the sum of Harga × Banyak.
  - The bill in Data Pembayaran is the drugs' rounded total plus the procedures' and examinations' totals.
- **Add Penunjang** (Penunjang tab) needs a Cabang. It opens "Data Tindakan" (`GET /api/pengobatan/penunjang`), which lists the supporting examinations that are active and offered at the branch, with the branch price (`refpenunjangdetail.hjual`) and their type (`refjenispanjang`). In lab, MED has 399 of them.
  - The list works like Tindakan without a doctor column.
  - A new Cabang empties it.
  - Totals are not rounded (lab: `tot_pnjng` = sum of Harga × Banyak).
- **Add Diagnosa** (Diagnosa tab) opens "Data Diagnosa" (`GET /api/pengobatan/icd`): the 18,505 ICD-X codes, paged and searched on the server, by ID, 10 rows per page.
  - It reuses the Master ICD query but needs only the Pengobatan menu.
  - Diagnoses are not per branch and carry no price.
  - A code already in the list cannot be picked again (lab stores only the code per line, `trxjualdiagnosa.icd_id`).
- **Pemeriksaan** tab: the examination of the picked registration, loaded from `trxpemeriksaan` (`GET /api/pengobatan/pemeriksaan/{idKun}`) and saved there with **Simpan Pemeriksaan** (`PUT`).
  - It saves one row per registration (insert, or update on `id_kun`), with Dokter PJ as `id_dok`, the user and the clinic time.
  - It saves on its own, before the sale itself is saved.
  - Fields: Keluhan Utama, Anamnesis, Tekanan Darah (sistole/diastole), Berat/Tinggi (whole numbers, as in the table), Suhu, Pemeriksaan Fisik, Rekap Hasil Penunjang, Hasil Penunjang, Rencana Penatalaksanaan, Hasil Pemeriksaan Dokter and Tindakan Dokter. Lengths and plausible values are checked.
  - Only registrations at the caller's branches (another branch's answers 404), and only while the visit is DAFTAR, PROSES or PAYMENT (409 otherwise).
  - The tab stays mounted, so typing survives a tab switch.
- **Save** (`POST /api/pengobatan`) records the sale in one transaction, following the lab data:
  - **What it checks:** the registration must be at the chosen branch, still DAFTAR, PROSES or PAYMENT, and not billed yet (409 otherwise).
  - **Prices:** they come from the master data at the branch; only an editable procedure price is taken from the form. A drug sold beyond the branch stock answers 409.
  - **trxjual:** `no_kwi` = YYMM + branch + 4-digit number per branch and month (e.g. 2611CTM0312; this rule matches all of lab), `tgl` = Tgl Kuitansi + the current clinic time, `flow_status` Close, and the patient's insurer.
  - **trxjualtotal:** tot_obat (drug lines rounded up to 500, PPN 10% for taxable drugs), tot_tndk and tot_pnjng (not rounded), `total`, `potongan` (Diskon), `pembayaran` (money handed over; empty Payment = exactly the amount due), `cicilan` (the settled part), `piutang` (what stays owed), `no_close` = YY + sale id, status 1.
  - **Lines:** `trxjualobat` (with the branch cost price and Dosis), `trxjualtindakan` (with the Dokter PJ), `trxjualpnjng` and `trxjualdiagnosa`.
  - **trxstock:** the branch stock goes down by the drugs sold. Lab does not show whether the legacy app did this.
  - **trxpendaftaran:** the registration becomes CLOSE.

  Save first opens the **Pembayaran** dialog: Jumlah (the bill), Diskon and Payment, with Close, Diskon (applies the discount to the form) and Bayar $ (saves). It previews Grand Total, Kembalian or Sisa. A Payment of 0 means paid exactly, and the Payment and Diskon fields of Data Pembayaran only show the result.
  The form then shows the receipt number and locks; **New** starts the next sale. Save refuses while the Pemeriksaan tab has unsaved typing, because a closed visit cannot change its examination. Tukar Point is disabled: points are never used in lab.
- **Kuitansi** (after Save) opens the "Kwitansi" dialog (`GET /api/pengobatan/kuitansi/{id}`).
  - It shows Tanggal, No Kwitansi, No RM / name, Sudah terima dari (the patient by default), Untuk Pembayaran, Biaya Pengobatan/Tindakan/Penunjang and Banyaknya uang (total minus discount).
  - **Print** (`POST /api/pengobatan/kuitansi/{id}/cetak`) records the print and opens `Kuitansi_<id>.pdf` in a new tab with the browser's print dialog, in the legacy "KWITANSI PENGOBATAN" layout: amount in words (`src/components/terbilang.ts`), "Jakarta , 20 Nov 2026", the printing user (e-mail without the domain) and the print time.
  - The first print writes `trxjualcetak` (migration `038_kuitansi.sql`) with `no_cetak` = `refbranch.kode_kwt`-YY-MM-5 digits, numbered per code and month (MED prints BTA, MIT prints CKG). This matches all of lab. Reprints keep the first number.
  - "Sudah terima dari" and "Untuk Pembayaran" are only printed: lab stores them nowhere.
- **Print ▾** (after Save) records the print like the Kuitansi (the first print gets the `no_cetak`), loads the sale with its lines (`GET /api/pengobatan/nota/{id}`) and prints one of four PDFs (`src/pages/notaPdf.ts`):
  - **A5 Reguler**: the legacy "BUKTI PEMBAYARAN" receipt (`/Print/Reguler`):
    - the print number, the branch, its company (`refbranch.nama_npwp`), address and phone;
    - No Transaksi (`no_kwi/dd/mm/yyyy/id`), patient, No RM, Penanggung (`refmpasien.penanggung`), Kasir, Asuransi, Pelayanan and Dokter;
    - every line with its amount, then Sub Total, Diskon (when any), Grand Total, Pembayaran, Kembalian, Piutang (when any) and Total Point (`refpasienpoint.point`, the patient's balance);
    - the print time from the server.
  - **A5 Bold**: the same in bold;
  - **Thermal**: the bill on an 80 mm roll;
  - **A5 Summary**: only the cost per group and the payment.

  Printing (`src/components/printPdf.ts`): the click opens a new tab right away (popup blockers allow it only during the click), and the PDF is shown there with the print dialog open, like the legacy `/Print/Reguler`. If the tab is blocked, the PDF is downloaded instead.

  Thermal and A5 Summary are a first version until legacy samples are available. The branches' address, phone and company were filled from lab where they were empty.
- **Lines history:** migration `037_pengobatan_detail.sql` creates the four line tables (deleted with their sale), and `scripts/import-lab-pengobatan-detail.sh` copies them from lab: about 1.3 million lines in a minute, skipping the 104 lines whose sale no longer exists.
- **Row actions on the list** (Action column):
  - **Edit** opens `/transaksi-klinik/pengobatan/edit/{id}` (legacy `/PengobatanDetail/Edit/{id}`). It loads the sale (`GET /api/pengobatan/detail/{id}`) into the same form, read-only: registration, patient, the four tabs, payment, and a Kembali | Kuitansi | Print header. A cancelled sale shows who cancelled it, when and why. The tabs and the card toggles stay usable; only the inputs are locked (also after Save).
  - **Cancel** (sales not yet cancelled) opens the legacy "Cancel Pengobatan" dialog: "Apakah data No Trans <id> akan di cancel?" with a required Alasan (max 100 characters). Yes calls `POST /api/pengobatan/detail/{id}/cancel`:
    - `trxjualtotal` gets status 2 with `cancelby` (the e-mail without its domain), `cancelat` and `cancelreason`, like lab's 3,084 cancelled sales;
    - the amounts and lines are kept, and the registration stays CLOSE (as in lab);
    - the drugs sold go back into the branch stock, undoing what Save took out (confirmed with the clinic; lab keeps no stock history);
    - a second cancel answers 409.
  - Both follow branch authorization: another branch's sale answers 404.
- **Not built yet:** the other lookups (patient, insurer, OTC referrer) and changing a saved sale. The form says so in a banner.

The list replaces the legacy `spVTrxJualByPeriodeAuth8` procedure and its `VTrxJualWithAddress` view with one parameterized query in `internal/pengobatan`. The query joins only what the list shows; the view joined about 15 tables, including examination notes and addresses. It also drops `exec spUpdateRefMPasien_RT_RW`, which rewrote the patients' RT/RW on every list request.

Migration `034_pengobatan.sql` creates `refjenisbayar`, `trxjual` and `trxjualtotal` with the lab layout, and indexes for the date and branch filters. `035_pengobatan_keys.sql` adds a foreign key to the payment type, a unique receipt number, and the total-to-sale link. It adds no keys to patients, doctors, services or registrations: lab has 7 sales of deleted patients, 1 of a deleted doctor and 112 of deleted registrations, and the Dokter and Pendaftaran re-imports truncate those tables with CASCADE. There is no branch key either: 355 sales from 2020–2025 use codes no branch has (`MED_`, `MGM_`, `MIT_`, `PRM_`), so branch authorization hides them, as it did in lab.

```bash
LAB_PASSWORD=... scripts/import-lab-pengobatan.sh            # about 12 seconds; skips the 43 totals without a sale
LAB_PASSWORD=... scripts/import-lab-pengobatan.sh --replace
LAB_PASSWORD=... scripts/import-lab-pemeriksaan.sh            # examination notes and Dokter PJ (036_pemeriksaan.sql), about 5 seconds
LAB_PASSWORD=... scripts/import-lab-pengobatan-detail.sh      # sale lines (037_pengobatan_detail.sql), after import-lab-pengobatan.sh
LAB_PASSWORD=... scripts/import-lab-penerimaan-barang.sh      # goods receipts, lines, totals, suppliers (039_penerimaan_barang.sql)
```

The data is personal (who was treated where, and what they paid): keep it on machines allowed to hold it, and never commit it.

### Master Data Satu Sehat › Organisasi & Lokasi

`/master-data-satu-sehat/organisasi-lokasi` lists, per branch and poli, the SATUSEHAT Organization and Location the clinic uses (`reforganizationlocation`): Kode, Cabang, Poli (`refjenisrawat`), Organisasi, Lokasi, Organisasi ID and Lokasi ID. Like the original screen there is no add or delete: rows are fixed per branch and poli, and the pencil edits the names and the two SATUSEHAT IDs (empty, or a UUID). Users only see and edit the rows of their own branches (`refbranchauth`). `jenisrawatid` has no foreign key, because `import-lab-pendaftaran.sh` truncates `refjenisrawat` with CASCADE.

```bash
LAB_PASSWORD=... scripts/import-lab-organisasi-lokasi.sh            # migration 033_organization_location.sql creates the table
LAB_PASSWORD=... scripts/import-lab-organisasi-lokasi.sh --replace
```

## Transaksi Klinik › Penerimaan Barang (goods receipts)

`/transaksi-klinik/penerimaan-barang` mirrors the legacy `/GoodReceive` list. It shows the goods receipts at the user's branches, newest first.

- **Tables:** migration `039_penerimaan_barang.sql` creates four tables, with the legacy columns and types:
  - `trxfaktur`: the receipts;
  - `trxfakturobat`: their drug lines, deleted with their receipt;
  - `trxfakturtotal`: total, discount, paid and owed;
  - `refsupplier`: the suppliers.
- **Import:** `scripts/import-lab-penerimaan-barang.sh` copies them from lab: 16 suppliers, 7,855 receipts, 42,390 lines and 7,804 totals (51 receipts have no total).
- **Columns:**
  - No, No GR (`no_fak`), Tgl Trans (`tgl`, the entry date), Tgl GR (`tgl_fak`, the receipt date);
  - Supplier: "-" when `id_supp` is 0, as for 6,322 of the receipts;
  - Total (`trxfakturtotal.total`), Cabang, Action;
  - **Action is View only**: receipts are not edited.
- **Filters:**
  - the period of the entry date (default yesterday and today, H-1 to H);
  - Cari: No GR, Supplier (with "- (tanpa supplier)");
  - Cabang, search (No GR, supplier, branch or receipt number), sort, paging and export.
- **Add** opens the receipt form at `/transaksi-klinik/penerimaan-barang/new` (legacy `/GoodReceive/New`).
  - **Header:**
    - No Trans (filled on Save), Cabang\*, Distributor\*, No GR\* (max 15 characters) and Tanggal GR (today by default, not later);
    - Distributor is picked in the **Data Distributor** lookup (`refdistributorobat`, 122 rows by code).
  - **Daftar Obat:**
    - **Add Obat** opens **Data Obat**: the drugs stocked at the chosen branch (`trxstock`), with ID, Obat, Satuan (selling unit), Harga (the branch's buying price `trxstock.hbeli`) and Stok;
    - each row takes No Batch (default 0), Banyak (1), Exp Date (a year from today) and Disc % (0), and the tick adds it;
    - a line can be edited (No Batch, Banyak, Exp Date, Disc %) or removed;
    - Jumlah = Harga × Banyak × (1 − Disc/100), and the card shows the Total;
    - choosing another branch empties the list (prices and stock are per branch).
  - **Save** (`POST /api/penerimaan-barang`) runs in one transaction, following lab's data:
    - `trxfaktur`: tgl now, `tgl_fak` and `tgl_jtp` = Tanggal GR, `id_supp` 0, `id_jenis_byr` 1, the user (e-mail without its domain) as creator;
    - `trxfakturobat`: the server's buying price, `tot_qjb` = Banyak;
    - `trxfakturtotal`: total = Σ Jumlah (as for 99.9% of lab), fully paid, status 1, `no_close` = YYMM + id (2611 + 13809 = 261113809);
    - **the branch stock goes up by Banyak**. In lab this fits 63% of the stock rows, against 34% for Banyak × isi per box.
  - The form is then read-only with the new No Trans; **New** starts another; **Print** is not built yet.
  - Validation: the branch must be the user's, the drug stocked there, Banyak > 0, Disc 0–100, No Batch ≤ 20.
- **View** opens `/transaksi-klinik/penerimaan-barang/view/{id}` (`GET /api/penerimaan-barang/detail/{id}`).
  - It shows the same form, read-only, with No Trans, Cabang, Distributor, No GR, Tanggal GR and the drug lines.
  - A note gives who entered it and when, the supplier and the total.
  - The header has only Kembali and Print; Add Obat and the line actions are hidden.
  - Another branch's receipt, or one that does not exist, answers 404.
- **Not built yet:** Print.
- **Endpoints:** `GET /api/penerimaan-barang` and `/options`, guarded by the menu and limited to the caller's branches (package `backend/internal/penerimaan`).
- **Checked against lab:** 10 Oct – 8 Nov 2026 lists 261 receipts with the same total and the same latest rows (13809 MJ/ETC/08112026 875,000, …).

## Report Klinik › Raw Data Pengobatan

`/report-klinik/raw-data-pengobatan` mirrors the legacy `/RawDataPengobatan` report. It is the Pengobatan list with the patient's columns, read-only (no Add, Edit or Cancel).

- **Columns, in the legacy order:** No Trans, Tanggal, No Kuitansi, No RM, Pasien, Alamat, RT, RW, Kelurahan (`refkelurahan`), Tanggal Lahir, Jenis Kelamin (`refgender`), Dokter, Jenis Rawat, Asuransi (the sale's `trxjual.id_jamkes`), Pembayaran, Total, Cabang, Status, Alasan Cancel, Diskon (`trxjualtotal.potongan`) and Dibuat Oleh (`trxjual.create_user`).
- **Same query as the list:** date range (default yesterday and today, H-1 to H, like the other reports), Cari (Nama Pasien, Jenis Pelayanan, Status Kunjungan), Filter (Pembayaran, Status), Cabang, search, sort, paging and export (Copy, Excel, CSV, PDF, Print), limited to the caller's branches.
- **Endpoints:** `GET /api/report-klinik/raw-data-pengobatan` and `/options`. They run the Pengobatan list handlers but are guarded by the report's own menu, so a role can open the report without the Pengobatan menu.

## Report Klinik › Penjualan Obat

`/report-klinik/penjualan-obat` mirrors the legacy `/PenjualanObat` report: the drugs sold per day at the user's branches.

- **One row per clinic day, drug, price, branch and settlement status.** Banyak is summed over the sales (two sales of 10 make one row of 20). Lab has no procedure or view for this report; the grouping follows the legacy screen, which shows no receipt or patient.
- **Columns:** Tanggal, Obat, Jenis (`refjenisobat`), Satuan (selling unit), Harga (`trxjualobat.hjual`), Banyak, Cabang and Status. Jumlah (Harga × Banyak) is hidden; turn it on under Column visibility.
- **Sorting:** by drug name, then day and branch, like the legacy list. Every column can be sorted.
- **Filters:** date range (default yesterday and today), Cari (Nama Obat, Jenis Obat, Status), Cabang, search, paging and export.
- **Endpoints:** `GET /api/report-klinik/penjualan-obat` and `/options`, guarded by the report's menu and limited to the caller's branches. The code lives in `backend/internal/reportklinik`, the package for the Report Klinik pages.
- **Checked against the database:** for 7–8 Nov 2026 the report has 364 rows, and its Banyak (2,725) and Jumlah (12,397,575) totals equal the sums over `trxjualobat`. A month loads in about 0.2 s.

## Report Klinik › Penjualan Tindakan

`/report-klinik/penjualan-tindakan` mirrors the legacy `/PenjualanTindakan` report. It shows every procedure line sold at the user's branches, newest first.

- **Columns:**
  - Tanggal, Tindakan, Jenis (`refjenistindakan`), Dokter, Pasien;
  - Fee Dokter, Harga Jual (`trxjualtindakan.hjual`), Banyak, Total Fee Dokter;
  - Cabang, Status.
- **Dokter** is the line's doctor (`trxjualtindakan.id_dok`), or the sale's Dokter PJ when the line has none.
- **Fee Dokter** is the branch's fee for the procedure, `reftindakandetail.hbeli` (unique per procedure and branch).
  - This matches every fee on the legacy screen (for example 21,500 at PRM and 23,750 at CTM for the same consultation).
  - Sales do not store a fee, so the report always uses the fee currently in the master data.
  - **Total Fee Dokter** = Fee Dokter × Banyak.
- **Filters:** date range (default yesterday and today), Cari (Nama Tindakan, Dokter, Status), Cabang, search, sort, paging and export.
- **Endpoints:** `GET /api/report-klinik/penjualan-tindakan` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:** for 7–8 Nov 2026 there are 235 lines, sales of 10,267,500 and doctor fees of 3,732,420, equal to direct sums over `trxjualtindakan`.

## Report Klinik › Penjualan Penunjang

`/report-klinik/penjualan-penunjang` mirrors the legacy `/PenjualanPenunjang` report: every supporting examination sold at the user's branches, newest first.

- **Columns:**
  - Tanggal, No RM, Nama, Penunjang;
  - Jenis (`refjenispanjang`, e.g. LABORATORIUM CV, LAB MOHC);
  - Dokter (the sale's Dokter PJ);
  - Harga (`trxjualpnjng.hjual`), Banyak;
  - Cabang, Status, Dibuat Oleh (`trxjual.create_user`).
- **Filters:**
  - date range (default yesterday and today);
  - Cari: Nama Penunjang, Jenis Penunjang, Status;
  - Cabang, search, sort, paging and export.
- **Endpoints:** `GET /api/report-klinik/penjualan-penunjang` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - 7–8 Nov 2026: 15 lines, 1,426,500;
  - October 2026: 310 lines, 25,728,500;
  - both equal direct sums over `trxjualpnjng`. A month loads in about 0.04 s.

## Report Klinik › Penjualan OTC

`/report-klinik/penjualan-otc` mirrors the legacy `/PenjualanOTC` report. It lists every drug line of type **OTC** (`refjenisobat`) sold at the user's branches, newest sale first.

- **Columns:**
  - No Trans, Tanggal, No Kuitansi, Obat, Jenis;
  - Perujuk (`refperujukotc`, via `trxjual.id_perujuk_otc`);
  - Banyak, Harga (`trxjualobat.hjual`), Ppn 11% (`trxjualobat.ppn`);
  - Total (Harga × Banyak + PPN);
  - Cabang, Status.
- **Which lines:**
  - Lines are chosen by drug type, not by referrer: sales with a referrer also hold ETIKAL and SOFT DRINK drugs, which the legacy screen does not show here (soft drinks have their own report).
  - An OTC line of a sale without a referrer is listed with Perujuk "-" (42 of 142 lines on 7–8 Nov 2026).
  - PPN is 0 on every stored line so far.
- **Filters:**
  - date range (default yesterday and today);
  - Cari: Nama Obat, Perujuk, Status;
  - Cabang, search (drug, receipt, referrer, branch or sale id), sort, paging and export.
- **Endpoints:** `GET /api/report-klinik/penjualan-otc` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - 7–8 Nov 2026: 142 lines, 3,099,668;
  - October 2026: 2,064 lines, 46,343,074;
  - both equal direct sums over `trxjualobat`. A month loads in about 0.05 s.

## Report Klinik › Penjualan Softdrink

`/report-klinik/penjualan-softdrink` mirrors the legacy `/PenjualanSoftdrink` report. It is Penjualan OTC for the drugs of type **SOFT DRINK**, with the same columns, filters and referrer.

- **Code:**
  - the backend runs the OTC query with the drug type as a bound parameter (`JenisOTC` / `JenisSoftDrink` in `reportklinik/otc.go`);
  - the page is `PenjualanOTCPage` with `softdrink`.
- **Endpoints:** `GET /api/report-klinik/penjualan-softdrink` and `/options`, guarded by the report's own menu.
- **Checked against the database:**
  - 7–8 Nov 2026: 15 lines, 124,307;
  - October 2026: 183 lines, 1,780,814;
  - the OTC report's figures are unchanged.

## Report Klinik › Pasien Baru

`/report-klinik/pasien-baru` mirrors the legacy `/PasienBaru` report. It lists the visits of patients registered in the period at the user's branches, newest first.

- **What counts:** a sale (`trxjual`) in the period whose patient was registered (`refmpasien.create_date`) in the same period. A new patient with two visits has two rows, as on the legacy screen.
- **Columns:**
  - Tanggal Kunjungan (the sale's date and time), No RM, Nama Pasien, Jenis Rawat;
  - Tempat Lahir, Tanggal Lahir, Umur (years on the visit day), Alamat;
  - Tanggal Daftar (the day the patient was registered);
  - Dokter (the sale's Dokter PJ), Status (the sale's), Cabang.
- **Filters:**
  - date range (default yesterday and today), for both the visit and the registration;
  - Cari: Nama Pasien, Jenis Rawat, Status;
  - Cabang, search, sort, paging and export.
  - Sorting by Umur sorts by birth date the other way round.
- **Endpoints:** `GET /api/report-klinik/pasien-baru` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - 7–8 Nov 2026: 63 visits by 59 new patients;
  - October 2026: 1,487 visits by 1,295 new patients;
  - both equal a direct count. A month loads in about 0.06 s.

## Report Klinik › Rekap Asuransi

`/report-klinik/rekap-asuransi` mirrors the legacy `/ReportAsuransi` report. It lists the insured sales at the user's branches, newest first.

- **Which sales:** those whose insurer (`trxjual.id_jamkes`) is not NON ASURANSI.
- **Rows per sale:**
  - the first row carries Tanggal, No RM, Card Number (`trxjual.no_jamkes`), Perusahaan (the insurer), Pasien, Dokter, Diagnosa (ICD names), Cabang and Status;
  - then the items, grouped under Keterangan Obat, Tindakan and Penunjang, with Details, Banyak, Harga and Total (Harga × Banyak). An empty group still has one row with zeros, as in the legacy report;
  - a Total row closes the sale (`trxjualtotal.total`, the sum of its items).
- **Paging:** it counts sales (10 sales per page with all their rows), whereas the legacy page cut a sale across pages. The export lists every row. This uses `pageEntries`, a new DataTable option for an entry shown as several rows.
- **Filters:**
  - date range (default yesterday and today, H-1 to H);
  - Cari: Nama Pasien, Perusahaan, Status;
  - Cabang, search and sort by the sale's columns.
- **Endpoints:** `GET /api/report-klinik/rekap-asuransi` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - 2–8 Nov 2026: 47 sales, 166 drug lines, 61 procedure lines and 53 diagnoses, total 5,027,999;
  - October 2026: 235 sales, total 29,307,409;
  - both equal direct counts. A month loads in about 0.02 s.

## Report Klinik › Transaksi Harian

`/report-klinik/transaksi-harian` mirrors the legacy `/ReportHarian` report. It lists every sale at the user's branches, newest first.

- **Columns:**
  - Tanggal, No RM, Pasien, Pelayanan, Dokter, Pembayaran;
  - Obat, Tindakan, Penunjang (`trxjualtotal.tot_obat`, `tot_tndk`, `tot_pnjng`), Total;
  - Cabang, Status, Diskon (`potongan`).
- **Filters:**
  - date range (default yesterday and today);
  - **Pelayanan** in the toolbar (ALL or one service), as on the legacy screen;
  - Cari: Nama Pasien, Pembayaran, Status;
  - Cabang, search, sort, paging and export.
- **Endpoints:** `GET /api/report-klinik/transaksi-harian` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - 7–8 Nov 2026: 235 sales, obat 12,400,000, tindakan 10,267,500, penunjang 1,426,500, total 24,094,000, diskon 714,325;
  - October 2026: 4,126 sales, total 490,283,008;
  - both equal direct sums, and the procedure and examination totals agree with Penjualan Tindakan and Penjualan Penunjang. A month loads in about 0.04 s.

## Report Klinik › All Stok

`/report-klinik/all-stok` mirrors the legacy `/ReportAllStock` report. It lists every drug at every branch of the user (`trxstock`), the dearest first.

- **Columns:**
  - Periode, Kode, Obat, Satuan Beli, Isi Per Box, Satuan Jual;
  - Harga Beli and Harga Jual (the branch's `trxstock.hbeli`, `hjual`);
  - Jenis, Principal, Distributor, Kode Moving PO;
  - Stok Awal, Jual Obat, Stok Akhir, Cabang.
- **Stock movement:** the app changes stock through sales (Save takes drugs out, Cancel puts them back) and goods receipts (Penerimaan Barang adds them). Stock adjustments are not recorded yet. Going back from today's stock:
  - Stok Akhir = current stock + the drugs sold after the period − the drugs received after it;
  - Jual Obat = the drugs sold in the period, cancelled sales left out;
  - Stok Awal = Stok Akhir + Jual Obat − the drugs received in the period.
  - The page says so in a note.
  - The receipts are imported from lab (`scripts/import-lab-penerimaan-barang.sh`). Lab's adjustments (`trxstokadjus`) are not: even with them only about 75% of lab's current stock can be rebuilt from its history.
  - For October 2026: Stok Awal 472,554 and Stok Akhir 481,125, with 62,614 received in the period and 5,638 after it, equal to a direct sum.
- **Filters:** period (default yesterday and today), Cari (Nama Obat, Jenis Obat, Kode Moving PO), Cabang, search (drug, type, principal, distributor, branch or drug id), sort, paging and export.
- **Endpoints:** `GET /api/report-klinik/all-stok` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:** for 7–8 Nov 2026 and for October 2026, over all 24,570 rows:
  - Jual Obat equals the non-cancelled sales in the period (2,664 and 54,043);
  - Stok Akhir equals the current stock plus the sales after it;
  - Stok Awal = Stok Akhir + Jual Obat on every row.
  - A page loads in about 0.2 s.

## Report Klinik › Rujukan

`/report-klinik/rujukan` mirrors the legacy `/ReportRujukan` report: the sales of external doctors at the user's branches, newest first.

- **Which sales:** the Dokter PJ is an external doctor, `refmdokter.id_internal = 3` (EKSTERNAL in `refdokterinternal`; 1 is -, 2 INTERNAL).
- **Rows:**
  - one per supporting examination of the sale; Total is that examination's Harga × Banyak;
  - a sale without examinations has one row with an empty Penunjang and Total 0, as on the legacy screen.
- **Columns:** Tanggal, No RM, Pasien, Dokter Eksternal, Jenis Rawat, Penunjang, Total, Cabang, Status.
- **Filters:**
  - date range (default yesterday and today, H-1 to H) and 20 rows a page, as in the legacy report;
  - Cari: Nama Pasien, Dokter Eksternal, Status;
  - Cabang, search, sort and export.
- **Endpoints:** `GET /api/report-klinik/rujukan` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:** for 9 Oct – 8 Nov 2026 there are 781 rows from 767 sales, 86 with an examination, total 7,801,500, equal to a direct count. A page loads in about 0.03 s.

## Report Klinik › Pendaftaran

`/report-klinik/pendaftaran` mirrors the legacy `/ReportPendaftaranKlinik` report: the registrations (`trxpendaftaran`) at the user's branches, newest first.

- **Columns:**
  - No Pendaftaran, No Antrian (`no_antri`), Tanggal / Waktu (`tgl_masuk`), Pelayanan;
  - Status Kun (`refstatustransaksi`: CLOSE, DAFTAR, PROSES, PAYMENT, CANCEL, NON KUN);
  - No RM, Nama Pasien, Umur (years on the registration day), Alamat, Cabang.
- **Filters:**
  - date range (default yesterday and today, H-1 to H);
  - Cari: Nama Pasien, Pelayanan, Status Kunjungan;
  - Cabang, search (also by registration number), sort, paging and export.
- **Endpoints:** `GET /api/report-klinik/pendaftaran` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - 7–8 Nov 2026: 246 registrations (CLOSE 214, DAFTAR 28, PROSES 4);
  - October 2026: 4,805;
  - both equal a direct count per status. A month loads in about 0.03 s.

## Report Klinik › Report Pendapatan

`/report-klinik/report-pendapatan` mirrors the legacy `/ReportPendapatan` screen. It has no table: a **Data Pendapatan** form downloads the "Laporan Pendapatan Klinik" PDF.

- **Form:**
  - Cabang (required, the user's branches);
  - Layanan (All or one service);
  - Dokter (All or one doctor): the doctors of the chosen branch (`refmdokterauth`, by id). It is disabled until a branch is chosen and goes back to All when the branch changes;
  - Dari Tanggal / Sampai Tanggal (default yesterday and today, H-1 to H);
  - **Clear** resets the form, **Download** saves `Report_Income.pdf` (`src/pages/pendapatanPdf.ts`).
- **The PDF (A4, as the legacy one):**
  - header "Klinik" with the branch name and phone, a double rule, the title;
  - Nama Poli, Dokter, Tanggal Posting (today), Petugas (the user, e-mail without its domain) and Periode;
  - a row per sale, oldest first: No, Tanggal / Waktu (with milliseconds), No Kwt, Nama Pasien, Obat, Tindakan, Penunjang, Total. The header is on the first page only and no row is cut across pages;
  - the summary:
    - "Jakarta, <date>", Mengetahui and the user;
    - Jumlah Pasien, Pasien Baru (sales whose patient was registered in the period) and Pasien Lama;
    - Total Obat, Tindakan and Penunjang, then Total Tagihan, Potongan and Total Pendapatan (Tagihan − Potongan).
- **Cancelled sales are left out**: they are no income.
- **Endpoints**, guarded by the report's menu:
  - `GET /api/report-klinik/pendapatan?cabang=…` (400 without a branch, 403 for another branch's);
  - `/options`;
  - `/dokter?cabang=…`.
- **Checked against the database:** for KLINIK CITRA MEDIKA on 7–8 Nov 2026 the PDF lists 66 sales (19 new and 47 returning patients), obat 3,690,500, tindakan 2,522,500, penunjang 69,000, total 6,282,000, equal to a direct sum.

## Report Klinik › Report Diagnosa

`/report-klinik/report-diagnosa` mirrors the legacy `/ReportDiagnosa` report. It lists every diagnosis (`trxjualdiagnosa`) of the sales at the user's branches, newest sale first; a sale with two diagnoses has two rows.

- **Columns:**
  - No Trans, Tanggal (date and time), No RM, Pasien, Tanggal Lahir, Usia (years on the visit day), Jenis Kelamin;
  - Dokter, Jenis Rawat, ICD.X and Diagnosa (`reficd`);
  - Resep Obat: the sale's drugs as "NAMA (banyak) , NAMA (banyak)", built for the page's rows only;
  - Keluhan Utama (`trxpemeriksaan.keluhan_utama` of the visit), Saturasi Oksigen, Cabang.
- **Saturasi Oksigen** is shown as "-". Neither this database nor lab stores it: the legacy app added it later.
- **Filters:**
  - date range (default yesterday and today, H-1 to H), 5 rows a page as in the legacy report;
  - Cari: Nama Pasien, ICD.X / Diagnosa (code or name), Jenis Rawat;
  - Cabang, search, sort and export.
- **Endpoints:** `GET /api/report-klinik/diagnosa` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - 7–8 Nov 2026: 77 diagnoses of 68 sales, 75 with a prescription and 57 with a complaint;
  - October 2026: 1,652 diagnoses of 1,392 sales;
  - both equal a direct count. A month loads in about 0.03 s.

## Report Klinik › Report ICD Dinkes

`/report-klinik/report-icd-dinkes` mirrors the legacy `/ReportDinkes` report: the cases of one branch per ICD code and service, for the health office (Dinas Kesehatan).

- **What counts as a case:** a diagnosis (`trxjualdiagnosa`) of a non-cancelled sale in the period. They are grouped by ICD code (`reficd`) and service (Jenis Rawat).
- **Columns:**
  - Kode, ICD, Jenis Rawat;
  - the age groups, by the patient's age on the visit day: 0-7 Hari, 8-28 Hari, 1-11 Bulan, 1-4, 5-9, 10-14, 15-19, 20-44, 45-59 and >59 Tahun;
  - Laki-laki and Perempuan (`refgender` 1 and 2), and Total.
  - A patient without a birth date is in no age group but is in the total.
- **Filters:** the period (default yesterday and today, H-1 to H) and **Cabang** in the toolbar (one branch, the first of the user's by default), as on the legacy screen; search (ICD code or name, service), sort (highest code first by default), paging and export.
- **Endpoints:** `GET /api/report-klinik/icd-dinkes` and `/options`, guarded by the report's menu and limited to the caller's branches.
- **Checked against the database:**
  - KLINIK CITRA MEDIKA, 7–8 Nov 2026: 16 cases in 8 lines (3 male, 13 female);
  - October 2026: 401 cases at CTM and 292 at KMJ;
  - all equal a direct count, every case falls in exactly one age group. A month loads in about 0.03 s.

## How the 15-minute inactivity logout works

The token lifetime (`JWT_TTL`) is also the inactivity window, and the rule is enforced on both the server and the client:

1. **Server:** every token expires `JWT_TTL` after it is issued. An expired token gets `401` on protected routes.
2. **Client:** the app records the last user activity (mouse, keyboard, scroll, touch). While you are active it calls `/api/auth/refresh` (at most every ~30 s), so the token's expiry stays about 15 minutes after your latest activity.
3. If there is no activity for `JWT_TTL`, the client logs you out and shows a message. By then the token has also expired on the server, so it cannot be reused.

Activity and logout are shared across browser tabs through `localStorage`. To test quickly, set `JWT_TTL=1m` in `.env`, run `docker compose up --build`, log in, and leave the page alone for a minute.

## Local development without Docker

```bash
docker compose up -d db                                # just the database

cd backend
export DATABASE_URL=postgres://app:app@localhost:5432/authdb?sslmode=disable
export JWT_SECRET=$(openssl rand -hex 32)
go run ./cmd/server

cd frontend
npm install
npm run dev                                            # http://localhost:5173, proxies /api to :8080
```

## Tests

```bash
cd backend && go test ./...
# or without a local Go install:
docker run --rm -v "$PWD/backend:/src" -w /src golang:1.26-alpine go test ./...
```

The tests cover JWT generation and parsing (expired tokens, wrong secret, `alg=none`), register and login validation, and the full HTTP flow (register, duplicate email, wrong password, login, `/api/me`, refresh, missing or expired token) against an in-memory repository. They also cover menu filtering by role and the role and user APIs (403 without the menu, validation, duplicate names, deleting a role in use, and the self-lockout guards).

## Deployment (GitHub Actions → server)

[.github/workflows/ci-cd.yml](.github/workflows/ci-cd.yml) runs on every push and pull request:

1. **Backend**: `go vet` and `go test -race`.
2. **Frontend**: `npm ci` and `npm run build` (type check included).
3. **Images** (push to `main` only): builds both Dockerfiles and pushes them to GitHub Container Registry as `ghcr.io/<owner>/<repo>-backend` and `-frontend`, tagged with the commit SHA and `latest`.
4. **Deploy** (after the images): copies [deploy/docker-compose.prod.yml](deploy/docker-compose.prod.yml) and [deploy/remote-deploy.sh](deploy/remote-deploy.sh) to the server over SSH, pulls the new images and restarts. The job fails if the backend healthcheck does not pass within 3 minutes.

The server runs only the backend and frontend containers. The database is the PostgreSQL installed on the server, and the migrations run when the backend starts.

### One-time server setup

On a Linux server with Docker Engine and the Compose plugin:

1. **Deploy user**: create a user that may run Docker (`sudo usermod -aG docker deploy`) and add the public half of a new SSH key pair to its `~/.ssh/authorized_keys`.
2. **PostgreSQL**: create the database and a user for the app. The containers connect from the Docker networks, so in `postgresql.conf` set `listen_addresses = '*'` (or the Docker gateway addresses), and in `pg_hba.conf` add `host new_rme rme 172.16.0.0/12 scram-sha-256`. Then restart PostgreSQL. Keep port 5432 closed to the internet in the firewall.
3. **Settings**: as the deploy user `mkdir -p /home/deploy/apps/new_rme`, copy [deploy/.env.example](deploy/.env.example) to `/home/deploy/apps/new_rme/.env`, fill in `DATABASE_URL` and a new `JWT_SECRET` (`openssl rand -hex 32`), and run `chmod 600 /home/deploy/apps/new_rme/.env`. The secrets stay on the server, not in GitHub.
4. **Data**: to copy the lab data, run `scripts/import-lab-all.sh` with `DATABASE_URL` pointing at the server's database, or restore a `pg_dump`.

### GitHub settings

In the repository's **Settings › Environments**, create an environment named `production` (optionally with required reviewers, so each deploy waits for approval). Add these secrets to it:

| Secret | Value |
|---|---|
| `SSH_HOST` | Server IP or host name |
| `SSH_USER` | The deploy user, e.g. `deploy` |
| `SSH_PRIVATE_KEY` | The private half of the deploy key |
| `SSH_KNOWN_HOSTS` | Output of `ssh-keyscan -p 22 <host>`, checked against the server's real fingerprint |

These variables are optional: `SSH_PORT` (default `22`) and `DEPLOY_PATH` (default `/home/deploy/apps/new_rme`).

The workflow pulls images with its own short-lived `GITHUB_TOKEN`, so the server needs no registry password.

### Rollback and operations

```bash
cd /home/deploy/apps/new_rme
docker compose ps                     # status
docker compose logs -f backend        # logs
IMAGE_TAG=<older-commit-sha> docker compose up -d   # roll back to an earlier build
```

Re-running an older workflow run from the Actions tab also deploys that commit again. Migrations only move forward, so roll back only to builds whose migrations are already applied.

## Project layout

```
backend/
  cmd/server/main.go          wiring, HTTP server, graceful shutdown
  internal/config             environment variables
  internal/db                 connection with retry and embedded SQL migrations (each runs once)
  internal/user               users repository (PostgreSQL)
  internal/auth               JWT, validation, handlers, middleware and tests
  internal/menu               menus repository, tree building, role filtering, GET /api/menus, RequireMenu
  internal/role               roles CRUD and user role assignment (/api/roles, /api/users)
  internal/branch             clinic branches in refbranch (/api/branches)
  internal/obat               drug master and stock per branch (/api/obat)
  internal/tindakan           procedures and prices per branch (/api/tindakan)
  internal/penunjang          supporting examinations and prices per branch (/api/penunjang)
  internal/pasien             patients with server-side paging, No RM generation (/api/pasien)
  internal/dokter             doctors, their procedures and practice branches (/api/dokter)
  internal/jamkes             health insurers, PKS terms and item lists (/api/jamkes)
  internal/distributor        drug distributors (/api/distributor)
  internal/principal          drug principals (/api/principal)
  internal/icd                ICD-X codes with server-side paging (/api/icd)
  internal/lab                lab examinations, Master Data > Lab (/api/lab)
  internal/labmaster          lab examination groups, Master Data > Lab Master (/api/lab-master)
  internal/labsubmaster       lab examination subgroups, Master Data > Lab Sub Master (/api/lab-sub-master)
  internal/doktermcu          MCU doctors, Master Data > Dokter (/api/dokter-mcu)
  internal/doktermaster       kinds of MCU doctor, Master Data > Dokter Master (/api/dokter-master)
  internal/nonlab             non-lab services, Master Data > Non Lab (/api/non-lab)
  internal/mastermcu          MCU examination groups, Master Data > MCU (/api/master-mcu)
  internal/pasienmcu          MCU patients, Master Data > Pasien (/api/pasien-mcu)
  internal/perusahaan         client companies, Master Data > Perusahaan (/api/perusahaan)
  internal/perujukotc         OTC referrers, "Penjual OTC" (/api/perujuk-otc)
  internal/pendaftaran        patient registrations, queue numbers, cancel (/api/pendaftaran)
  internal/pengobatan         treatment sales list and the form's lookups (registration, doctor, drug, procedure, examination, ICD-X) and the examination save (/api/pengobatan)
  internal/branchauth         which branches a user is mapped to (refbranchauth), GET /api/me/branches
  internal/httpx              shared JSON response helpers
  internal/docs               OpenAPI spec and Swagger UI (SWAGGER_ENABLED)
frontend/
  src/api.ts                  fetch wrapper
  src/auth/AuthContext.tsx    session, inactivity timer, token refresh
  src/layout/                 app shell: sidebar from /api/menus, top bar, icons
  src/components/             DataTable (paging, search, sort, column visibility) and its exporters
  src/components/branchView   per-branch columns, branch filter (Obat, Tindakan, Penunjang)
  src/components/FormKit      modal form, fields and amount inputs for master-data dialogs
  src/components/barcode.ts   Code 128 barcode (SVG) for patient cards
  src/components/dates.ts     today, addDays, DD/MM/YYYY formatting and age
  src/pages/                  Register, Login, Dashboard, RolePage, UserPage, BranchPage, ObatPage, TindakanPage, PenunjangPage, PasienPage, DokterPage, JamkesPage, DistributorPage, PrincipalPage, IcdPage, PerujukOtcPage, MasterMcuPage, PerusahaanPage, PasienMcuPage, LabMasterPage, LabSubMasterPage, LabPage, NonLabPage, DokterMasterPage, DokterMcuPage, PendaftaranPage, PengobatanPage, PengobatanFormPage (+ PendaftaranLookup, DokterLookup, ObatLookup, TindakanLookup, DiagnosaLookup, PemeriksaanTab, pengobatanCalc), MenuPage (placeholder)
scripts/import-lab-obat.sh    copies drugs and stock from the legacy lab database
scripts/import-lab-tindakan.sh copies procedures and prices per branch from lab
scripts/import-lab-penunjang.sh copies supporting examinations and prices per branch from lab
scripts/import-lab-pasien.sh  copies patients and their lookups from lab (personal data)
scripts/import-lab-dokter.sh  copies doctors, their procedures and practice branches from lab
scripts/import-lab-jamkes.sh  copies the insurers' PKS terms and item lists from lab
scripts/import-lab-icd.sh     copies ICD-X codes and their types from lab
scripts/import-lab-perujuk-otc.sh copies the OTC referrers from lab
scripts/import-lab-pendaftaran.sh copies registrations, services, statuses and point balances from lab (personal data)
scripts/import-lab-pengobatan.sh copies treatment sales, their totals and payment types from lab (personal data)
scripts/import-lab-pemeriksaan.sh copies examination notes per registration from lab (medical data)
scripts/import-lab-pengobatan-detail.sh copies the sale lines (drugs, procedures, examinations, diagnoses) and receipt prints from lab (medical data)
scripts/import-lab-penerimaan-barang.sh copies the goods receipts, their lines and totals, and the suppliers from lab
scripts/import-lab-organisasi-lokasi.sh copies the SATUSEHAT organizations & locations from lab
scripts/lib/lab.sh            shared helpers for the import scripts
  nginx.conf                  serves the SPA and proxies /api to the backend
docker-compose.yml
.env.example
```

---

<details>
<summary>Original assignment</summary>

### Take-home Assignment: Auth with JWT

Build a small application in **TypeScript/Go/C#** that supports user **registration** and **login** using **JWT**.

- **Register**: fields `email`, `password`, `confirmPassword`
- **Login**: input `email`, `password`; returns a **JWT** containing at least `email` and a user id
- **Authenticated view / endpoint**: shows `Hello [email], welcome back`
- The user should be logged out after 15 minutes of inactivity

Deliver a runnable project and a README explaining how to build and run it (Docker Compose) and the required environment variables.

Acceptance criteria: registration works with validation; login returns a usable JWT; a protected route or screen shows the welcome message using JWT auth.

Optional bonus: Docker, a Go backend, a React/Vue frontend, tests.

</details>
