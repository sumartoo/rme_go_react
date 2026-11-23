-- Master Data > Dokter: the MCU doctors, each of a kind (refdoctormaster),
-- with their fee percentage. Separate from the clinic doctors (refdokter).
-- Same columns, types and primary key as refdoctor in the legacy "lab"
-- database. Data is loaded with scripts/import-lab-dokter-mcu.sh.
-- doctormaster points to refdoctormaster (not enforced, like lab).
CREATE TABLE IF NOT EXISTS refdoctor (
    doctorcode    varchar(20) NOT NULL,
    doctorname    varchar(50),
    doctormaster  integer,
    feepercentage double precision,
    CONSTRAINT refdoctor_pkey PRIMARY KEY (doctorcode)
);
