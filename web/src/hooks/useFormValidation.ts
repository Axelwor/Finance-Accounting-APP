import { useState, useCallback } from "react";

type Validator = (value: unknown) => string | null;
type Validators = Record<string, Validator>;

export function useFormValidation(validators: Validators) {
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});

  const validateField = useCallback(
    (name: string, value: unknown) => {
      const validator = validators[name];
      if (!validator) return null;
      const error = validator(value);
      setErrors((prev) => ({ ...prev, [name]: error || "" }));
      return error;
    },
    [validators],
  );

  const validateAll = useCallback(() => {
    const newErrors: Record<string, string> = {};
    let hasErrors = false;

    for (const [name, validator] of Object.entries(validators)) {
      // We can't get current values here, so this is for schema-level validation
      // Field-level validation should be done via validateField
      const currentValue = "";
      const error = validator(currentValue);
      if (error) {
        newErrors[name] = error;
        hasErrors = true;
      }
    }

    setErrors(newErrors);
    setTouched((prev) => {
      const allTouched = { ...prev };
      Object.keys(validators).forEach((key) => {
        allTouched[key] = true;
      });
      return allTouched;
    });

    return !hasErrors;
  }, [validators]);

  const clearErrors = useCallback((field?: string) => {
    if (field) {
      setErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
      setTouched((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    } else {
      setErrors({});
      setTouched({});
    }
  }, []);

  const markTouched = useCallback((field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  }, []);

  const onFieldBlur = useCallback(
    (name: string, value: unknown) => {
      markTouched(name);
      validateField(name, value);
    },
    [markTouched, validateField],
  );

  const onFieldChange = useCallback((name: string) => {
    setErrors((prev) => {
      const next = { ...prev };
      delete next[name];
      return next;
    });
  }, []);

  return {
    errors,
    touched,
    validate: validateAll,
    validateField,
    clearErrors,
    markTouched,
    onFieldBlur,
    onFieldChange,
  };
}

export type ValidationReturn = ReturnType<typeof useFormValidation>;
