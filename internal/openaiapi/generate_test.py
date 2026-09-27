import unittest

import yaml
from generate import SpecLoader, prepare_discriminators


class GenerateTest(unittest.TestCase):
    def test_discriminator_mapping(self) -> None:
        references = {
            "#/components/schemas/Comparison": {
                "properties": {"type": {"enum": ["eq", "ne"]}}
            },
            "#/components/schemas/Compound": {
                "properties": {"type": {"enum": ["and", "or"]}}
            },
        }
        schema = {
            "oneOf": [{"$ref": ref} for ref in references],
            "discriminator": {"propertyName": "type"},
        }
        prepare_discriminators(schema, references)
        self.assertEqual(
            schema["discriminator"]["mapping"],
            {
                "eq": "#/components/schemas/Comparison",
                "ne": "#/components/schemas/Comparison",
                "and": "#/components/schemas/Compound",
                "or": "#/components/schemas/Compound",
            },
        )

    def test_ambiguous_discriminator(self) -> None:
        references = {
            f"#/components/schemas/{name}": {
                "properties": {"type": {"const": "message"}}
            }
            for name in ("Input", "Output")
        }
        schema = {
            "oneOf": [{"$ref": ref} for ref in references],
            "discriminator": {"propertyName": "type"},
        }
        with self.assertRaises(ValueError):
            prepare_discriminators(schema, references)

    def test_missing_discriminator_values(self) -> None:
        schema = {
            "anyOf": [{"$ref": "#/components/schemas/Untagged"}],
            "discriminator": {"propertyName": "type"},
        }
        with self.assertRaises(ValueError):
            prepare_discriminators(
                schema, {"#/components/schemas/Untagged": {"type": "object"}}
            )

    def test_yaml_scalars(self) -> None:
        values = yaml.load(
            '[true, 42, 1.5, null, 2024-10-01, 2024-10-01T12:30:00Z, "001"]',
            Loader=SpecLoader,
        )
        self.assertEqual(
            values,
            [True, 42, 1.5, None, "2024-10-01", "2024-10-01T12:30:00Z", "001"],
        )


if __name__ == "__main__":
    unittest.main()
