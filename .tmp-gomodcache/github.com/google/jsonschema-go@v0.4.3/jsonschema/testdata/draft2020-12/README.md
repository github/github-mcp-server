# JSON Schema test suite for 2020-12

These files were copied from
https://github.com/json-schema-org/JSON-Schema-Test-Suite/tree/83e866b46c9f9e7082fd51e83a61c5f2145a1ab7/tests/draft2020-12.

The following files were omitted:

content.json: it is not required to validate content fields
(https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-00#rfc.section.8.1).

format.json: it is not required to validate format fields (https://json-schema.org/draft/2020-12/draft-bhutton-json-schema-validation-00#rfc.section.7.1). 

vocabulary.json: this package doesn't support explicit vocabularies, other than the 2020-12 draft.

The "optional" directory: this package doesn't implement any optional features.
