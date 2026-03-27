#!/usr/bin/env python3
"""Generate a single merged AMR parquet test fixture for atb-cli AMR tests.

Creates:
  fixtures/amrfinderplus.parquet  (15 rows: 10 AMR + 5 STRESS, all Genus=Escherichia)

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


def generate():
    FIXTURES_DIR.mkdir(parents=True, exist_ok=True)

    # 10 AMR rows + 5 STRESS rows, all for Escherichia
    names = [
        # AMR rows
        "SAMN00000001", "SAMN00000001",
        "SAMN00000002", "SAMN00000002",
        "SAMN00000003",
        "SAMN00000004",
        "SAMN00000005", "SAMN00000005",
        "SAMN00000011",
        "SAMN00000019",
        # STRESS rows
        "SAMN00000001",
        "SAMN00000002",
        "SAMN00000003",
        "SAMN00000011",
        "SAMN00000019",
    ]
    gene_symbols = [
        # AMR
        "blaTEM-1", "acrF",
        "blaTEM-1", "aadA1",
        "blaEC",
        "blaTEM-1",
        "acrF", "aadA1",
        "blaOXA-1",
        "blaCTX-M-15",
        # STRESS
        "emrA", "tolC", "ompT", "emrA", "tolC",
    ]
    hierarchy_nodes = [
        # AMR
        "blaTEM", "AcrF",
        "blaTEM", "AadA",
        "blaEC", "blaTEM",
        "AcrF", "AadA",
        "blaOXA", "blaCTX-M",
        # STRESS
        "EmrA", "TolC", "OmpT", "EmrA", "TolC",
    ]
    element_types = ["AMR"] * 10 + ["STRESS"] * 5
    element_subtypes = [
        # AMR
        "BETA-LACTAM", "EFFLUX",
        "BETA-LACTAM", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
        "EFFLUX", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
        # STRESS
        "ACID", "ACID", "ACID", "ACID", "ACID",
    ]
    coverage = [
        # AMR
        100.0, 98.5,
        100.0, 99.2,
        100.0,
        97.8,
        98.5, 100.0,
        95.0,
        100.0,
        # STRESS
        100.0, 99.5, 98.0, 100.0, 97.5,
    ]
    identity = [
        # AMR
        100.0, 97.3,
        99.8, 98.5,
        100.0,
        97.8,
        97.3, 98.5,
        93.5,
        99.5,
        # STRESS
        100.0, 98.8, 97.2, 100.0, 99.0,
    ]
    methods = [
        # AMR
        "EXACTX", "BLASTX",
        "BLASTX", "EXACTX",
        "EXACTX",
        "PARTIAL",
        "BLASTX", "EXACTX",
        "PARTIAL",
        "EXACTX",
        # STRESS
        "EXACTX", "EXACTX", "BLASTX", "EXACTX", "EXACTX",
    ]
    classes = [
        # AMR
        "BETA-LACTAM", "EFFLUX",
        "BETA-LACTAM", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
        "EFFLUX", "AMINOGLYCOSIDE",
        "BETA-LACTAM", "BETA-LACTAM",
        # STRESS
        "STRESS", "STRESS", "STRESS", "STRESS", "STRESS",
    ]
    subclasses = [
        # AMR
        "PENICILLIN", "EFFLUX",
        "PENICILLIN", "STREPTOMYCIN",
        "PENICILLIN", "PENICILLIN",
        "EFFLUX", "STREPTOMYCIN",
        "PENAM", "CEPHALOSPORIN",
        # STRESS
        "ACID", "ACID", "ACID", "ACID", "ACID",
    ]
    species = ["Escherichia coli"] * 15
    genus = ["Escherichia"] * 15

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
        "Genus": s(genus),
    })

    out_path = FIXTURES_DIR / "amrfinderplus.parquet"
    pq.write_table(table, out_path)
    print(f"amrfinderplus.parquet: {len(table)} rows")
    print(f"Written to {out_path}")


if __name__ == "__main__":
    generate()
    print(f"\nAMR fixture written to {FIXTURES_DIR}")
