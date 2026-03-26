#!/usr/bin/env python3
"""Generate small AMR parquet test fixtures for atb-cli AMR tests.

Creates:
  fixtures/amr/amr_by_genus/Genus=Escherichia/data_0.parquet  (10 rows)
  fixtures/amr/stress_by_genus/Genus=Escherichia/data_0.parquet  (5 rows)

Column types use regular pa.string() (NOT large_string) to match real AMR data.
"""

import pyarrow as pa
import pyarrow.parquet as pq
from pathlib import Path

SCRIPT_DIR = Path(__file__).parent
FIXTURES_DIR = SCRIPT_DIR / "fixtures"


def s(vals) -> pa.Array:
    """Cast list to regular string array (not large_string)."""
    return pa.array(vals, type=pa.string())


def generate_amr():
    out_dir = FIXTURES_DIR / "amr" / "amr_by_genus" / "Genus=Escherichia"
    out_dir.mkdir(parents=True, exist_ok=True)

    names = [
        "SAMN00000001", "SAMN00000001",
        "SAMN00000002", "SAMN00000002",
        "SAMN00000003",
        "SAMN00000004",
        "SAMN00000005", "SAMN00000005",
        "SAMN00000011",
        "SAMN00000019",
    ]
    gene_symbols = [
        "blaTEM-1", "acrF",
        "blaTEM-1", "aadA1",
        "blaEC",
        "blaTEM-1",
        "acrF", "aadA1",
        "blaOXA-1",
        "blaCTX-M-15",
    ]
    hierarchy_nodes = [
        "blaTEM", "AcrF",
        "blaTEM", "AadA",
        "blaEC", "blaTEM",
        "AcrF", "AadA",
        "blaOXA", "blaCTX-M",
    ]
    element_types = ["AMR"] * 10
    element_subtypes = [
        "BETA-LACTAM", "EFFLUX",
        "BETA-LACTAM", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
        "EFFLUX", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
    ]
    coverage = [
        100.0, 98.5,
        100.0, 99.2,
        100.0,
        97.8,
        98.5, 100.0,
        95.0,
        100.0,
    ]
    identity = [
        100.0, 97.3,
        99.8, 98.5,
        100.0,
        97.8,
        97.3, 98.5,
        93.5,
        99.5,
    ]
    methods = [
        "EXACTX", "BLASTX",
        "BLASTX", "EXACTX",
        "EXACTX",
        "PARTIAL",
        "BLASTX", "EXACTX",
        "PARTIAL",
        "EXACTX",
    ]
    classes = [
        "BETA-LACTAM", "EFFLUX",
        "BETA-LACTAM", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
        "EFFLUX", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
    ]
    subclasses = [
        "PENICILLIN", "EFFLUX",
        "PENICILLIN", "STREPTOMYCIN",
        "PENICILLIN", "PENICILLIN",
        "EFFLUX", "STREPTOMYCIN",
        "PENAM", "CEPHALOSPORIN",
    ]
    species = ["Escherichia coli"] * 10

    table = pa.table({
        "Name": s(names),
        "Gene symbol": s(gene_symbols),
        "Hierarchy node": s(hierarchy_nodes),
        "Element type": s(element_types),
        "Element subtype": s(element_subtypes),
        "% Coverage of reference sequence": pa.array(coverage, type=pa.float64()),
        "% Identity to reference sequence": pa.array(identity, type=pa.float64()),
        "Method": s(methods),
        "Class": s(classes),
        "Subclass": s(subclasses),
        "Species": s(species),
    })

    pq.write_table(table, out_dir / "data_0.parquet")
    print(f"amr_by_genus/Genus=Escherichia/data_0.parquet: {len(table)} rows")


def generate_stress():
    out_dir = FIXTURES_DIR / "amr" / "stress_by_genus" / "Genus=Escherichia"
    out_dir.mkdir(parents=True, exist_ok=True)

    names = [
        "SAMN00000001",
        "SAMN00000002",
        "SAMN00000003",
        "SAMN00000011",
        "SAMN00000019",
    ]
    gene_symbols = ["emrA", "tolC", "ompT", "emrA", "tolC"]
    hierarchy_nodes = ["EmrA", "TolC", "OmpT", "EmrA", "TolC"]
    element_types = ["STRESS"] * 5
    element_subtypes = ["ACID", "ACID", "ACID", "ACID", "ACID"]
    coverage = [100.0, 99.5, 98.0, 100.0, 97.5]
    identity = [100.0, 98.8, 97.2, 100.0, 99.0]
    methods = ["EXACTX", "EXACTX", "BLASTX", "EXACTX", "EXACTX"]
    classes = ["STRESS", "STRESS", "STRESS", "STRESS", "STRESS"]
    subclasses = ["ACID", "ACID", "ACID", "ACID", "ACID"]
    species = ["Escherichia coli"] * 5

    table = pa.table({
        "Name": s(names),
        "Gene symbol": s(gene_symbols),
        "Hierarchy node": s(hierarchy_nodes),
        "Element type": s(element_types),
        "Element subtype": s(element_subtypes),
        "% Coverage of reference sequence": pa.array(coverage, type=pa.float64()),
        "% Identity to reference sequence": pa.array(identity, type=pa.float64()),
        "Method": s(methods),
        "Class": s(classes),
        "Subclass": s(subclasses),
        "Species": s(species),
    })

    pq.write_table(table, out_dir / "data_0.parquet")
    print(f"stress_by_genus/Genus=Escherichia/data_0.parquet: {len(table)} rows")


if __name__ == "__main__":
    generate_amr()
    generate_stress()
    print(f"\nAll AMR fixtures written to {FIXTURES_DIR}")
