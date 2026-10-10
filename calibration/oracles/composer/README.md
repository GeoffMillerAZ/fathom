# composer oracle

`composer_oracle.php` is the oracle for the `composer` doc-engine dialect (LB-Q-composer). Tier A: the tool is
Composer itself, and it shares no code with fathom's extractor.

| Reads | Tier | Emits |
|---|---|---|
| Composer 2.10.3's root package of each `composer.json` (`Factory::create`) | A | definitions, references |

## What is Composer's

Every name. `Factory::create()` loads a manifest through `RootPackageLoader`, the loader every Composer command
uses; the program asks the resulting package for its pretty name, its scripts (`getScripts()`, the set
`composer run-script --list` prints), and its require and require-dev links. Which script commands are
references to another script is Composer's own rule, `EventDispatcher::isComposerScript()` (a command beginning
`@` that is not `@php ...` or `@putenv ...`, read with reflection), minus `@composer ...` (which Composer runs as
a command), and only a script the package defines (Composer warns "a reference to a non-existent script"
otherwise). A manifest Composer refuses is named on stderr and left out of `--files-out`.

## What is not

Lines. Composer prints no position, so the program places a script on the line its key is written on with a
small JSON scanner (`jsonKeyLines`) and the package at line 1 (the entity is the manifest itself, as the cargo
oracle places its package). The scanner decides no name. A dependency is spelled as the manifest spells it:
Composer lower-cases a link's target.

## Cross-checks run by hand, not scored (2026-10-09, composer 2.10.3, scratch copies)

- `composer validate --no-check-publish` is valid on laravel/laravel, symfony/demo and all 40 manifests of the
  sparse laravel/framework checkout.
- `composer show --tree --locked` on symfony/demo (the only subject with a `composer.lock`): its 47 top-level
  packages are exactly the 47 non-platform packages `require` and `require-dev` name, so the references the
  oracle emits for them are the packages Composer locks.
- `composer dump-autoload --classmap-authoritative --no-scripts` generates 6 classes for laravel/laravel and 35
  for symfony/demo, from the psr-4 directories the manifests declare. The dialect's autoload references (a
  `repo-path` to each psr-4, psr-0, classmap and files entry) are the L3 family, not scored here: they resolve to
  a file only when a directory holds one doc file, and Composer's own resolution of a class to its file
  (`autoload_classmap.php`) is the oracle a later L3 reading would use (LB-Q-composer-rest).

## Run

```sh
composer_phar=<scratch>/composer.phar        # a copy of the installed composer phar
COMPOSER_HOME=<toolcache> php -d display_errors=stderr calibration/oracles/composer/composer_oracle.php \
  --phar $composer_phar --subject <checkout> --files-out <scratch>/composer-<subject>-files.txt \
  > <scratch>/composer-<subject>.json
indexbench lang-oracle -dialect composer -subject <checkout> -subject-name <owner/repo> \
  -tool-oracle <scratch>/composer-<subject>.json -out docs/bench/readings/lang/composer.json
```

Versions measured (2026-10-09): Composer 2.10.3 (2026-08-27), PHP 8.5.11.
