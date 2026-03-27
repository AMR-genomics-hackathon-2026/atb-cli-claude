Jane wants
Get me all AMRF+ results for species-X
Note from jane: would be good to be able to specify a filter here (eg high quality only)
Get me all the MLST for species X

https://github.com/immem-hackathon-2025/atb-amr-shiny 
AMR parquet files is in https://github.com/immem-hackathon-2025/atb-amr-shiny/tree/main/data/amr_by_genus

Other ideas:
Get me all the ST131 E coli
Get me 100 evenly spread Salmonella
Get me this genome
Get me the closest genome to my genome
Get me <all these results> for <this set of genomes: species, MLST, list?>
Return the command to download them?
Get me all info on sample X
Get me all genomes for species X
Restrict searches by high quality, chekm2, etc etc
High level stats: number of genomes, number per species etc

(out of scope?: How does my sample compare with it)


Shing’s thoughts/questions about CLI
CLI commands:
fetch: Fetch a parquet database from an URL (constant and version-specific).
update
query: Query the database by a user-specified set of filters (in a TOML file) to get a set of sample IDs/accessions (dump to CSV for further analysis?).
summarise: Generate summary statistics of the samples (number of total samples, available collection dates, available geographical location, sequencing platform, etc.; customisable) in the CSV file.
download: Download the sequences for a set of sample IDs/accessions in a CSV file.
Pipeline to do query/summarise/download in a single command. (Limit to a max number of samples?).
Version control
Tied to a specific version of ATB/incremental updates and the parquet database.
What to name this CLI?
atb-cli
Goals:
Make queries reproducible.
The query process should be well documented.
data schema location: ~/atb/metadata/parquet. This parquet files contain metadata of the data/genome as well as ulr