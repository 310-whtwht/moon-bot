-- Script strategies: a version of type `script` (in `code`) carries its rules
-- as Starlark source. NULL for built-in strategy types.
ALTER TABLE strategy_versions
    ADD COLUMN script MEDIUMTEXT NULL AFTER code;
