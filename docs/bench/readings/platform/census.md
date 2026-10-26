# PLT-M0a: the framework census inventory

The inventory half of PLT-M0 (`docs/plans/platform-engineering.md`, owner ruling of 2026-10-26: PLT-M0 splits).
This file lists items only. It classifies nothing: every row of `census-items.csv` carries the literal class
`unclassified` and an empty `fact_kinds`. The classification runs later through the decision port with a named
provider, a 20% owner audit and Cohen's kappa (the api lane, needs LAB-34). The verdict line belongs to that lane
and is not written here.

PLT-M0 inventory: 411 items

## What is stored

`census-items.csv` columns: `framework,pillar,item_id,short_title,url,class,fact_kinds,retrieved`. Sorted by
framework, pillar and item id (C collation), LF line endings, no quoting (no field carries a comma or a quote).
Per item the census keeps the publisher's id, a short title of at most 8 words, and a URL. A publisher title longer
than 8 words is replaced by our own wording. No description, question, best-practice text or checklist sentence
is copied.

## Sources

The framework hosts (docs.aws.amazon.com, learn.microsoft.com, docs.cloud.google.com, sre.google) did not resolve
from the session that built this census. Every source below is therefore a pinned public mirror read through git,
or a web-search listing of the publisher's pages. Each row says which.

| Framework | Source read | Pin | Framework version | Retrieved | Licence as read at retrieval |
|---|---|---|---|---|---|
| AWS Well-Architected (`aws-well-architected`) | The six pillar whitepaper PDFs and a best-practice list (`ecs_fargate_app/well_architected_docs/`) in github.com/aws-samples/well-architected-iac-analyzer | `b933408f557fcda03baec11ca303bff77eeeec8a` | Each PDF: "Publication date: June 27, 2024"; URLs use the matching `wellarchitected/2024-06-27/framework/` path | 2026-10-26 | PDFs: "Copyright © 2024 Amazon Web Services, Inc. and/or its affiliates. All rights reserved." Mirror repository: MIT No Attribution (`LICENSE`) |
| Azure Well-Architected (`azure-well-architected`) | `WARP/WAF Category Descriptions.csv` (ids and captions) and `WARP/Azure_Well_Architected_Review_Sample.csv` (ids and guide URLs) in github.com/Azure/WellArchitected-Tools | `cd7e629b230e78fd556170dd11265175b358d239` (commit date 2026-04-20) | The checklist ids current in that file; the review sample's header is dated Nov 17, 2023 | 2026-10-26 | Mirror repository: MIT (`LICENSE`, Microsoft Corporation). The learn.microsoft.com page licence was not read. |
| Google Cloud Well-Architected Framework (`google-well-architected`) | Web-search listings of each pillar page and its principle pages (titles and URLs) | none (no repository) | Pillar realignments named in the framework's "What's new" listing: reliability 2024-12-30, performance 2024-12-06, security 2025-02-05 | 2026-10-26 | Not read: the host did not resolve. |
| Google SRE production readiness review (`google-sre-prr`) | A web-search listing of chapter 32, "The Evolving SRE Engagement Model", of the SRE book | none | The book as published | 2026-10-26 | Not read: the host did not resolve. |

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
| azure-well-architected | operational-excellence | 12 |
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

Totals: AWS 308, Azure 60, Google 37, PRR 6. The KEEP bar's denominator is "the three frameworks' items", so it
excludes or includes the 6 PRR areas. The classification lane states which before it reads the share.

## Completeness proof

For each framework and pillar, the CSV count must equal the count of ids on the pillar's index, and a second
extraction from the per-item pages must yield the same id set.

| Framework | Index extraction | Per-item extraction | Result |
|---|---|---|---|
| AWS | ids in each pillar PDF's table of contents (the text before "Publication date") | ids at body headings, plus the PDF's named destinations (one per best-practice page), plus the best-practice list's rows | id sets equal in every pillar except one (see discrepancies); all three per-item counts equal the CSV in all six pillars |
| Azure | ids in `WAF Category Descriptions.csv` (contiguous `01` to `N` per pillar) | ids in the review sample | 46 of 60 confirmed by the second extraction (see discrepancies) |
| Google | principle titles listed by a web search of each pillar page | principle pages found by URL in separate web searches | 37 of 37 found by URL; both are search-mediated (see discrepancies) |
| PRR | one search-mediated listing of the chapter | none possible | 6, unconfirmed by a second extraction |

To reproduce the checks, run from the repository root with poppler's `pdfinfo` and `pdftotext`. Scratch files go
under `workspace/tmp`.

```sh
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
  toc=$(pdftotext -layout $f - | sed '/Publication date/q' | grep -oE "\b$x[0-9]{2}-BP[0-9]{2}\b" | sort -u | wc -l)
  heads=$(pdftotext -layout $f - | grep -oE "^ *$x[0-9]{2}-BP[0-9]{2} " | tr -d ' ' | sort -u)
  dests=$(pdfinfo -dests $f | grep -c "\"${s}_")
  csv=$(awk -F, -v p=$n '$1=="aws-well-architected" && $2==p {print $3}' $CSV | sort -u)
  [ "$heads" = "$csv" ] && same=yes || same=NO
  echo "$n index=$toc headings=$(echo "$heads" | wc -l) dests=$dests csv=$(echo "$csv" | wc -l) headings=csv:$same"
done
grep -c . $D/wellarchitected_best_practices.csv   # 309: a header and 308 rows

# Azure: clone the mirror at its pin
git clone -q https://github.com/Azure/WellArchitected-Tools workspace/tmp/census/azure
git -C workspace/tmp/census/azure checkout -q cd7e629b230e78fd556170dd11265175b358d239
W=workspace/tmp/census/azure/WARP
grep -oE '^[A-Za-z ]+,(RE|SE|CO|OE|PE):[0-9]{2},' "$W/WAF Category Descriptions.csv" | cut -d, -f2 | sort -u > workspace/tmp/census/az-index
grep -oE ',(RE|SE|CO|OE|PE):[0-9]{2},' "$W/Azure_Well_Architected_Review_Sample.csv" | tr -d , | sort -u > workspace/tmp/census/az-items
awk -F, '$1=="azure-well-architected"{print $3}' $CSV | sort -u | diff - workspace/tmp/census/az-index && echo index=csv
comm -13 workspace/tmp/census/az-index workspace/tmp/census/az-items | wc -l   # 0: no id outside the index
comm -23 workspace/tmp/census/az-index workspace/tmp/census/az-items | tr '\n' ' '  # the 14 below
```

Readings on 2026-10-26: AWS cost-optimization index 50, headings 50, destinations 50, CSV 50. Operational
excellence: index 62, then 68, 68 and 68. Performance-efficiency: 32 in all four. Reliability: 67 in all four.
Security: 63 in all four. Sustainability: 28 in all four. Headings equal the CSV in all six pillars. The
best-practice list has 308 rows. Azure: the index equals the CSV (60), and the review sample confirms 46 with none
outside the index.

## Count discrepancies

1. **AWS operational excellence, index 62 against 68.** The OPS PDF's table of contents omits OPS02-BP01 to
   OPS02-BP06. All six have body headings, named destinations and rows in the best-practice list. The CSV carries 68.
2. **AWS framework version.** This census reads the 2024-06-27 release. A search result reported a 2025 framework
   update that added best practices, but its index could not be read from this session. The counts above are for
   2024-06-27 only, and no total is quoted for a later release. The classification lane re-pins to the current
   release or keeps this one, and records which.
3. **Azure, 14 ids not confirmed per item.** The review sample confirms 46 of the 60 index ids. These 14 have no
   second extraction, and their URL is the pillar checklist page, not a guide page: CO:06, CO:08, CO:09, CO:11, CO:12,
   CO:13, CO:14, OE:09, OE:12, PE:02, PE:08, PE:09, PE:10, RE:08. The index counts per pillar (RE 10, SE 12, CO 14,
   OE 12, PE 12) could not be compared against the live checklist pages.
4. **Google, both extractions search-mediated.** The pillar pages and principle pages were not read directly. Each
   principle was found as a page URL in a web search, and each pillar's principle count is as stated in that
   pillar's search listing (reliability 9, security 7, cost 4, operational excellence 5, performance 4,
   sustainability 8). Pages under a pillar path that the pillar does not list as a principle are excluded. For
   example, `security/shared-responsibility-shared-fate` is excluded. The publisher gives no framework-wide total,
   and none is quoted.
5. **PRR, single extraction.** The six areas come from one search-mediated listing of chapter 32 in source order.
   No per-item pages exist and the chapter was not read directly. The `PRR-<n>` ids are minted.

## Classification rule (for the api lane)

The rule is applied per item through the decision port, kind `choice`, with a named provider. The owner audits 20%,
and the reading is void under Cohen's kappa 0.6.

- `code-answerable`: every part of the item is answerable at a commit by a fact kind fathom stores or a WAE- row
  plans. Example to verify at classification: REL05-BP03 (retry limits).
- `partly`: some part needs a human or runtime data. Example: OPS08-BP04 (actionable alerts).
- `attestation-only`: no part is answerable from code. Example: REL13-BP01 (recovery objectives).

`partly` is reported, never counted toward the KEEP bar. The `fact_kinds` column is filled for `code-answerable`
and `partly` items. Until then every row reads `unclassified`, so no session label primes the audit.
