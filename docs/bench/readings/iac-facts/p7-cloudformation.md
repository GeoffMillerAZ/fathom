IF-P7 verdict: PASS with one condition: size and key fit the design, but the schema files carry no licence that allows redistribution, so catalogs are built locally and publishing one needs an owner decision.

## Plain-language summary

- Size: the us-east-1 file is a 3.16 MiB zip of 1,876 JSON files, 14.67 MiB unzipped. That is under the design's 16 MiB bound for one catalog version (92% of it), with the largest single file at 340 KiB against the 1 MiB blob bound and a JSON depth of 13 against 64.
- Regions are not interchangeable. Of the 6 other regions read, none matched us-east-1. Entry counts run from 878 (me-central-1) to 1,725 (eu-west-1 and ap-southeast-2) against 1,876, and the share of shared types that differ in content runs from 2% (eu-west-1) to 50% (me-central-1). A catalog must be keyed by region, as the design says.
- Version key: no version number comes with the file. A digest works. Hash the zip's bytes only if nothing rebuilds it, because the zip embeds its own build time in every entry. A digest over each entry's name and body is stable under a rebuild that changes nothing, and is what the catalog should use.
- Licence: the files carry no licence, and the terms that do apply forbid distributing and deriving from AWS content unless a separate licence says otherwise. None was found. A catalog may not be published on this evidence. It must be built locally by someone who downloaded the file themselves. Whether a catalog of extracted facts (paths and keys, no prose) counts as a derivative is a legal question these sources do not answer: **owner decision needed before any publishing.**

## Method

Date: 2026-11-04 to 05. Source: `https://schema.cloudformation.<region>.amazonaws.com/CloudformationSchema.zip`, the location the CloudFormation user guide page "CloudFormation resource provider schemas" links per region. Seven regions were fetched once each: us-east-1, eu-west-1, ap-southeast-2, sa-east-1, af-south-1, me-central-1, us-gov-west-1. A second fetch of us-east-1 returned the same bytes and the same ETag. The files stayed under a scratch directory and are not committed. Numbers come from a throwaway script (Python standard library, no product code).

## Size and fit against the design's bounds

| | us-east-1 |
|---|---|
| Zip bytes | 3,314,029 (3.16 MiB) |
| Entries | 1,876 (1,875 `AWS::*`, 1 `Alexa::*`), no `AWS::Serverless::*` |
| Unzipped bytes | 15,384,705 (14.67 MiB) |
| gzip of all bodies concatenated | 1,869,749 (1.78 MiB) |
| Largest entry | 348,246 bytes |
| Entries over 100 KB | 7 |
| Deepest JSON | 13 levels |
| Design bounds | 16 MiB per version, 1 MiB blob, 500,000 entries, depth 64 |

All four bounds hold. The 16 MiB bound is the tight one at 92%, before any pruning of prose. A catalog that keeps only the replacement and identity keys would be far smaller, but that is a build choice for IF-C1, not measured here.

Seven regions together hold 81.4 MB of entries but only 2,526 distinct bodies totalling 21.4 MB, because most types are identical across regions. Compressed, those distinct bodies take 2.9 MiB. A catalog that deduplicates blobs across regions therefore costs about 1.4 times one region unzipped, not 7 times.

## Regional differences, us-east-1 against each other region

Regions were fetched within a 3.5 hour window, one of them (me-central-1) about 3 hours earlier than the rest, so part of the difference is timing, not geography. All counts are exact.

| Region | Entries | Only in us-east-1 | Only in region | Shared types differing | `createOnlyProperties` set differs | `conditionalCreateOnlyProperties` set differs |
|---|---|---|---|---|---|---|
| eu-west-1 | 1,725 | 158 | 7 | 27 of 1,718 | 3 | 1 |
| ap-southeast-2 | 1,725 | 151 | 0 | 34 of 1,725 | 3 | 1 |
| sa-east-1 | 1,412 | 464 | 0 | 44 of 1,412 | 4 | 1 |
| af-south-1 | 1,150 | 727 | 1 | 62 of 1,149 | 8 | 2 |
| us-gov-west-1 | 1,022 | 855 | 1 | 164 of 1,021 | 31 | 4 |
| me-central-1 | 878 | 1,008 | 10 | 434 of 868 | 71 | 11 |

Across all seven regions 704 types appear everywhere and 309 of those are byte-identical.

What differs inside a shared type: handlers, tagging, definitions and property lists (for example, partition-aware ARN patterns such as `arn:(aws|aws-us-gov|aws-cn)` against `arn:aws[-a-z]*`). A replacement list changes too: `aws-budgets-budget` lists `/properties/Budget/BudgetName` as create-only in us-east-1 and not in eu-west-1. Some regional lists differ only by order (4 in sa-east-1, 10 in af-south-1), which a reader must compare as sets.

For the design: the replacement fact is regional. A template that names no region must read the catalog of the region the operator chooses, and the answer must say which. IF-C1 already keys by region. The numbers here say that keying is not optional.

## Digest as the version key

Observed on 7 zips: the zip contains per-entry modification times set about two minutes before the file's `Last-Modified`, and each region's file was built at a different time (six within 3.5 hours of each other, one about 3 hours earlier). The ETag is the zip's MD5. So a hash of the zip's bytes changes with any rebuild, and I could not observe whether a rebuild ever happens without a content change.

Recommendation, not a measurement: key the catalog by SHA-256 over the sorted list of (entry name, SHA-256 of entry body). It ignores zip metadata. For the seven files read:

| Region | Content digest (SHA-256, first 16 hex) | Zip SHA-256 (first 16 hex) |
|---|---|---|
| us-east-1 | 5ba095e5e8de043c | 02bbf530ab4897f9 |
| eu-west-1 | 6b95596fec798993 | a9218c97338c3e65 |
| ap-southeast-2 | 938baece5e79d4db | 9bf8e7940e4fa3bb |
| sa-east-1 | 656491b575691e24 | 826ccc2769b0d597 |
| af-south-1 | 5f0d433d1c536f68 | 2fd9ffcc582d878a |
| me-central-1 | a875954078f75e04 | 49ba9131c1e802c5 |
| us-gov-west-1 | 0d991eef4af8accc | c06211563b9a48da |

A second read after the next rebuild would show whether the content digest holds still when nothing changed. That is unmeasured.

## Licence

Facts first, then the reading.

1. **The file has no licence.** The zip holds only JSON schema files, with no licence, notice or readme. Three of those files mention "copyright", "licence" or "SPDX" only as property names in the schema (for example `SpdxLicenseId` in `AWS::ServerlessRepo::Application`). The schema host answers 403 at its root and at `/LICENSE`, `/license.txt`, `/NOTICE`, and the response headers carry no licence.
2. **The user guide offers the download without a licence.** Page "CloudFormation resource provider schemas" in the CloudFormation user guide says: "CloudFormation makes available the resource provider schemas for the AWS resources by Region ... To download the resource provider schemas for a specific Region, see the following table." (`https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resource-type-schemas.html`.) It states no terms.
3. **AWS Site Terms** (last updated June 4, 2025, `https://aws.amazon.com/terms/`), COPYRIGHT: "All content included on the AWS Site, such as text, graphics, logos, button icons, images, audio clips, digital downloads, data compilations, and software, is the property of AWS or its content suppliers". LICENSE AND SITE ACCESS: "Unless otherwise specified by AWS in a separate license, your right to use any software, data, documentation or other materials that you access or download through the AWS Site is subject to these Site Terms or, if you have an AWS account, the Agreement." It also excludes from the site licence "any resale or commercial use of the AWS Site or its contents; any derivative use of the AWS Site or its contents". The one open licence it grants is narrow: "The materials hosted on docs.aws.amazon.com are licensed as follows: documentation ... is licensed under CC-BY-SA-4.0, while any code therein is licensed under MIT-0." The schema files are hosted on `schema.cloudformation.<region>.amazonaws.com`, not on `docs.aws.amazon.com`, and they are neither documentation nor code in a guide, so that sentence does not name them.
4. **AWS Customer Agreement** (last updated October 29, 2026, `https://aws.amazon.com/agreement/`): "AWS Content" means "APIs, WSDLs, sample code, software libraries, command line tools, proofs of concept, templates, advice, information, programs ... and any other Content made available by us and our affiliates related to use of the Services or on the AWS Site and other related technology". Section 6.3: "The Intellectual Property License applies to your use of AWS Content and the Services."
5. **AWS Intellectual Property License** (last updated October 27, 2025, `https://aws.amazon.com/legal/aws-ip-license-terms/`), section 3: "a limited, royalty-free, revocable, non-exclusive, non-sublicensable, non-transferrable license to copy and use the AWS Content solely in connection with your permitted use of the Services". Section 4: neither you nor any End User will "(a) modify, distribute, alter, tamper with, repair, or otherwise create derivative works of any Content included in the Services or AWS Content (except to the extent such Content is provided to you under a separate license that expressly permits the creation of derivative works), or (b) sublicense the Services or AWS Content." Section 3 also says: "Some AWS Content and Third-Party Content may be provided to you under a separate license, such as the Apache License, Version 2.0, or other open source license."
6. **The AWS Service Terms** (updated November 3, 2026, `https://aws.amazon.com/service-terms/`) contain no CloudFormation-specific clause: the page was searched for the word and has none.
7. **No statement from AWS was found** on redistributing the zip: a GitHub issue search across public repositories for the zip's name and licence terms returned nothing.

Tolerance, not a grant (two precedents, both read as evidence of practice only):

- The Pulumi AWS-native provider repository (Apache-2.0) commits a directory named `aws-cloudformation-schema/` that holds 1,898 files named like the registry's schema files, last updated 2026-11-02. It carries no notice about their origin in that directory.
- `cfn-lint` (MIT-0, published by the `aws-cloudformation` GitHub organisation) ships per-region schema data inside its package. Its release workflow runs `cfn-lint --update-specs` and copies the result into the package. Its `NOTICE` file says only "cfn-flip" and the Amazon copyright line. MIT-0 covers the package's own software, and that package says nothing about relicensing the bundled data.

Reading. The default terms treat the schema files as AWS Content: a limited, non-sublicensable licence to copy and use them in connection with using the Services, and a bar on distributing them or creating derivative works, unless a separate licence says otherwise. No separate licence was found. Therefore:

- **Built locally: allowed.** An operator who has downloaded the file under the terms above and builds a catalog for their own use on their own machine fits the use the licence describes. The design already requires this: the product has no download path, and `cmd/iaccatalog` reads a local file.
- **Published: not on this evidence.** A catalog in a release or a repository is distribution. Whether it is a "derivative work" depends on what it keeps (a catalog of replacement paths and attribute names against one that copies descriptions) and on a legal reading these sources do not settle.
- **Decision owed by the owner:** ask AWS for written permission or a statement, or take counsel's reading, before any catalog derived from this file ships or is committed. Until then, do not commit the zip, any extract of it, or a built catalog, and do not add a catalog to a release.
- The plan row IF-C1 and the ADR-091 amendment (e) already allow for this: the builder ships, the operator builds. This reading confirms that branch and removes the "publish" branch for now.

I read these as primary-source texts, not legal advice. The pages were fetched on 2026-11-04 and will change; the dates above are the pages' own.

## Limits

One fetch per region, a few hours apart, and one snapshot in time, so rebuild cadence and digest stability are unmeasured. Regional differences are partly timing. Seven of the 37 regions listed on the guide page were read, and the two China regions were not tried. The sizes are for the raw file: no catalog format exists yet, so no pruned or compressed catalog size is claimed.

## What the next stage must know

- IF-C1 keys by source, region and content digest. Do not use the zip's digest.
- Never commit the zip, an extract or a built catalog from it until the owner decides.
- Replacement paths differ by region for 3 to 71 types per region read. An answer names the region and the digest.
- The registry file has no transform types such as `AWS::Serverless::*`, which matches the design's `unbound` rule.
