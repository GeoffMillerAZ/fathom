# PLT-M0a: the framework census inventory

The inventory half of PLT-M0 (`docs/plans/platform-engineering.md`, owner ruling of 2026-10-26: PLT-M0 splits).
This file lists items only. It classifies nothing: every row of `census-items.csv` carries the literal class
`unclassified` and an empty `fact_kinds`. The classification runs later through the decision port with a named
provider, a 20% owner audit and Cohen's kappa (the api lane, needs LAB-34). The verdict line belongs to that lane
and is not written here.

PLT-M0 inventory: 410 items

## What is stored

`census-items.csv` columns: `framework,pillar,item_id,short_title,url,class,fact_kinds,retrieved`. Sorted by
framework, pillar and item id (C collation), LF line endings, no quoting (no field carries a comma or a quote).
Per item the census keeps the publisher's id, a short title of at most 8 words, and a URL. A publisher title longer
than 8 words is replaced by our own wording. No description, question, best-practice text or checklist sentence
is copied.

AWS `short_title` basis: 243 of the 308 are the publisher's own best-practice titles (at most 8 words), which the
publisher also releases under MIT-0 in `wellarchitected_best_practices.csv` at `b933408`. The other 65 are our wording.
`item_id` and `url` (a dated path plus the PDF's named-destination slug) are facts. No question, description or body
text is stored.

## Sources

AWS is read from a pinned public mirror through git, because the AWS framework pages did not resolve from the
session that first built this census. Azure and Google were re-read directly from the live pages on 2026-10-26
(the Azure `ms.date` and the Google "Last updated" values are in the table). The Azure ids follow the live checklist
pages. The WARP file at the pin below supplied the first extraction and carries a superseded OE numbering
(discrepancy 3).

| Framework | Source read | Pin | Framework version | Retrieved | Licence as read at retrieval |
|---|---|---|---|---|---|
| AWS Well-Architected (`aws-well-architected`) | The six pillar whitepaper PDFs and a best-practice list (`ecs_fargate_app/well_architected_docs/`) in github.com/aws-samples/well-architected-iac-analyzer | `b933408f557fcda03baec11ca303bff77eeeec8a` | Each PDF: "Publication date: June 27, 2024"; URLs use the matching `wellarchitected/2024-06-27/framework/` path | 2026-10-26 | PDFs: "Copyright © 2024 Amazon Web Services, Inc. and/or its affiliates. All rights reserved." Mirror repository: MIT No Attribution (`LICENSE`); `wellarchitected_best_practices.csv` is under MIT-0 |
| Azure Well-Architected (`azure-well-architected`) | The five live checklist pages under `learn.microsoft.com/azure/well-architected/<pillar>/checklist` (ids), the guide pages they link (URLs), and `WARP/Azure_Well_Architected_Review_Sample.csv` as a second extraction | Page `ms.date` values: reliability 2023-11-15, security 2023-11-14, cost-optimization 2023-11-15, performance-efficiency 2026-04-20, operational-excellence 2026-02-11. WARP file: github.com/Azure/WellArchitected-Tools at `cd7e629b230e78fd556170dd11265175b358d239` | Live checklist pages | 2026-10-26 | WARP repository: MIT (`LICENSE`, Microsoft Corporation). The learn.microsoft.com page licence was not read. |
| Google Cloud Well-Architected Framework (`google-well-architected`) | The six live pillar pages under `docs.cloud.google.com/architecture/framework/<pillar>` and their principle links | none (no repository) | "Last updated" on each pillar page: reliability, security, cost-optimization, operational-excellence and sustainability 2026-10-13; performance-optimization 2026-10-22 | 2026-10-26 | CC BY 4.0 (code samples Apache-2.0), as stated on the page, read 2026-10-26 |
| Google SRE production readiness review (`google-sre-prr`) | The live chapter 32, "The Evolving SRE Engagement Model", of the SRE book | none | The book as published; the page carries no last-updated date | 2026-10-26 | CC BY-NC-ND 4.0, as stated on the page, read 2026-10-26 |

Item granularity, per the brief:

- **AWS**: one row per best practice (`REL05-BP03`). The URL is the best practice's own page. Its slug comes from
  the PDF's named destination at that best practice's heading.
- **Azure**: one row per checklist recommendation (`RE:01`). The URL is the guide page the recommendation links to,
  taken from the review sample. When the review sample does not name the id, the URL is the pillar's checklist page.
- **Google**: one row per pillar principle page. The id is the pillar slug and the page slug
  (`reliability/set-targets`). The pillar index pages say they map their recommendations to these principles.
- **PRR**: one row per review area, in source order. The id `PRR-<n>` is minted by this census and is not the
  publisher's. All six share the chapter URL, because the areas have no pages of their own.

## Per-pillar counts

| Framework | Pillar | Items |
|---|---|---|
| aws-well-architected | cost-optimization | 50 |
| aws-well-architected | operational-excellence | 68 |
| aws-well-architected | performance-efficiency | 32 |
| aws-well-architected | reliability | 67 |
| aws-well-architected | security | 63 |
| aws-well-architected | sustainability | 28 |
| azure-well-architected | cost-optimization | 14 |
| azure-well-architected | operational-excellence | 11 |
| azure-well-architected | performance-efficiency | 12 |
| azure-well-architected | reliability | 10 |
| azure-well-architected | security | 12 |
| google-sre-prr | prr | 6 |
| google-well-architected | cost-optimization | 4 |
| google-well-architected | operational-excellence | 5 |
| google-well-architected | performance-optimization | 4 |
| google-well-architected | reliability | 9 |
| google-well-architected | security | 7 |
| google-well-architected | sustainability | 8 |

Totals: AWS 308, Azure 59, Google 37 (404 framework rows), PRR 6 (410 inventory items). The PRR areas are reported
separately and never counted in the KEEP denominator, and the PLT-M0 bar reads on the pooled framework rows with each
framework's share beside it. Both are fixed in `docs/bench/prereg/platform.md` under H-M0 and the PLT-M0 bar.

## Completeness proof

For each framework and pillar, the CSV count must equal the count of ids on the pillar's index, and a second
extraction from the per-item pages must yield the same id set.

| Framework | Index extraction | Per-item extraction | Result |
|---|---|---|---|
| AWS | ids in each pillar PDF's table of contents (the text before "Publication date") | ids at body headings, plus the PDF's named destinations (one per best-practice page), plus the best-practice list's rows | id sets equal in every pillar except one (see discrepancies); all three per-item counts equal the CSV in all six pillars |
| Azure | ids on each live checklist page | ids in the review sample and in `WAF Category Descriptions.csv` | live checklist ids equal the CSV in all five pillars (59); 46 of 59 also confirmed by the review sample |
| Google | principle links on each live pillar page | each principle page returns HTTP 200 | 37 of 37 (see discrepancies) |
| PRR | the chapter's list of review areas, read directly | none possible | 6, single extraction |

To reproduce the checks, run the block below with bash (not zsh), from the repository root, with poppler's `pdfinfo` and `pdftotext`. Scratch files go
under `workspace/tmp`.

```bash
CSV=docs/bench/readings/platform/census-items.csv
# CSV: per-pillar counts, sort order, line endings, field count
awk -F, 'NR>1{c[$1","$2]++} END{for(k in c) print k, c[k]}' $CSV | sort
tail -n +2 $CSV | LC_ALL=C sort -c -t, -k1,1 -k2,2 -k3,3 && echo sorted
grep -c $'\r' $CSV            # 0
awk -F, 'NF!=8' $CSV | wc -l  # 0
awk -F, 'NR>1 && $6!="unclassified"' $CSV | wc -l  # 0

# AWS: clone the mirror at its pin
git clone -q https://github.com/aws-samples/well-architected-iac-analyzer workspace/tmp/census/aws
git -C workspace/tmp/census/aws checkout -q b933408f557fcda03baec11ca303bff77eeeec8a
D=workspace/tmp/census/aws/ecs_fargate_app/well_architected_docs
for p in cost-optimization:COST:cost operational-excellence:OPS:ops performance-efficiency:PERF:perf \
         reliability:REL:rel security:SEC:sec sustainability:SUS:sus; do
  IFS=: read n x s <<<"$p"; f=$D/wellarchitected-$n-pillar.pdf
  toc=$(pdftotext -layout $f - | sed '/Publication date/q' | grep -oE "\b${x}[0-9]{2}-BP[0-9]{2}\b" | sort -u | wc -l)
  heads=$(pdftotext -layout $f - | grep -oE "^ *${x}[0-9]{2}-BP[0-9]{2} " | tr -d ' ' | sort -u)
  dests=$(pdfinfo -dests $f | grep -c "\"${s}_")
  csv=$(awk -F, -v p=$n '$1=="aws-well-architected" && $2==p {print $3}' $CSV | sort -u)
  [ "$heads" = "$csv" ] && same=yes || same=NO
  echo "$n index=$toc headings=$(echo "$heads" | wc -l) dests=$dests csv=$(echo "$csv" | wc -l) headings=csv:$same"
done
grep -c . $D/wellarchitected_best_practices.csv   # 309: a header and 308 rows

# Azure: ids on the five live checklist pages (the index extraction)
for p in reliability:RE security:SE cost-optimization:CO performance-efficiency:PE operational-excellence:OE; do
  curl -sL "https://learn.microsoft.com/en-us/azure/well-architected/${p%%:*}/checklist" | grep -oE "\b${p##*:}:[0-9]{2}\b" | sort -u
done | sort -u > workspace/tmp/census/az-live
awk -F, '$1=="azure-well-architected"{print $3}' $CSV | sort -u | diff - workspace/tmp/census/az-live && echo csv=live  # 59

# Azure: clone the WARP mirror at its pin (second extraction)
git clone -q https://github.com/Azure/WellArchitected-Tools workspace/tmp/census/azure
git -C workspace/tmp/census/azure checkout -q cd7e629b230e78fd556170dd11265175b358d239
W=workspace/tmp/census/azure/WARP
grep -oE '^[A-Za-z ]+,(RE|SE|CO|OE|PE):[0-9]{2},' "$W/WAF Category Descriptions.csv" | cut -d, -f2 | sort -u > workspace/tmp/census/az-index
grep -oE ',(RE|SE|CO|OE|PE):[0-9]{2},' "$W/Azure_Well_Architected_Review_Sample.csv" | tr -d , | sort -u > workspace/tmp/census/az-items
comm -13 workspace/tmp/census/az-index workspace/tmp/census/az-items | wc -l   # 0: no id outside the index
comm -23 workspace/tmp/census/az-index workspace/tmp/census/az-items | tr '\n' ' '  # the 14 in the WARP file, 13 of them live
comm -23 workspace/tmp/census/az-index workspace/tmp/census/az-live  # OE:12, the superseded id
```

Readings on 2026-10-26: AWS cost-optimization index 50, headings 50, destinations 50, CSV 50. Operational
excellence: index 62, then 68, 68 and 68. Performance-efficiency: 32 in all four. Reliability: 67 in all four.
Security: 63 in all four. Sustainability: 28 in all four. Headings equal the CSV in all six pillars. The
best-practice list has 308 rows. Azure: the live checklist ids equal the CSV (59: RE 10, SE 12, CO 14, OE 11,
PE 12). The WARP index has one id the live pages lack (OE:12), and the review sample confirms 46 of the 59 with none
outside the index.

## Count discrepancies

1. **AWS operational excellence, index 62 against 68.** The OPS PDF's table of contents omits OPS02-BP01 to
   OPS02-BP06. All six have body headings, named destinations and rows in the best-practice list. The CSV carries 68.
2. **AWS framework version.** This census reads the 2024-06-27 release, pinned for reproducibility
   (`docs/bench/prereg/platform.md`, H-M0, which also carries the revisit trigger). The live document-revisions
   page lists a later major update dated 2024-11-06 and the `2025-02-25` framework path resolves. This census does not
   read it, and no total is quoted for it.
3. **Azure, the WARP file carries a superseded OE numbering.** The WARP file at `cd7e629` lists OE:01 to OE:12, with
   OE:09 "Task automation" and OE:12 "Failure mitigation". The live checklist (`ms.date` 2026-02-11) lists OE:01 to
   OE:11, with OE:09 about testing practices and no OE:12. The CSV follows the live page: OE:12 is dropped and OE:09
   is retitled. Titles for OE:02, OE:03, OE:04, OE:08 and OE:11 were also re-worded against the live text, and the
   URLs of OE:08 and OE:09 now point at their guide pages. RE, SE, CO and PE equal the live checklists. 13 live ids
   have no second extraction from the review sample: CO:06, CO:08, CO:09, CO:11, CO:12, CO:13, CO:14, OE:09, PE:02,
   PE:08, PE:09, PE:10, RE:08. Their URL is the pillar checklist page where no guide page is linked.
4. **Google, direct extraction.** Each of the six pillar pages was read directly and its principle links counted
   (reliability 9, security 7, cost 4, operational excellence 5, performance 4, sustainability 8), each principle page
   returning HTTP 200. Pages under a pillar path that the pillar does not list as a principle are excluded. For
   example, `security/shared-responsibility-shared-fate` is linked on the security page but is excluded. The publisher
   gives no framework-wide total, and none is quoted.
5. **PRR, single extraction.** The six areas come from the chapter's list of review areas, read directly, in source order. No
   per-item pages exist. The `PRR-<n>` ids are minted.

## Classification rule (for the api lane)

The rule is applied per item through the decision port, kind `choice`, with a named provider. The owner audits 20%,
and the reading is void under Cohen's kappa 0.6.

- `code-answerable`: every part of the item is answerable at a commit by a fact kind fathom stores or a WAE- row
  plans. Example to verify at classification: REL05-BP03 (retry limits).
- `partly`: some part needs a human or runtime data. Example: OPS08-BP04 (actionable alerts).
- `attestation-only`: no part is answerable from code. Example: REL13-BP01 (recovery objectives).

`partly` is reported, never counted toward the KEEP bar. The `fact_kinds` column is filled for `code-answerable`
and `partly` items. Until then every row reads `unclassified`, so no session label primes the audit.
