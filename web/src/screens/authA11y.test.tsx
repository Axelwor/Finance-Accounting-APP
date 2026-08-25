import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AuthScreen } from "./AuthScreen";
import { OnboardingScreen } from "./OnboardingScreen";

vi.mock("react-router-dom", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useNavigate: () => vi.fn(),
  useSearchParams: () => [new URLSearchParams(), vi.fn()],
  Link: ({ children }: { children: React.ReactNode }) => <a href="#">{children}</a>,
}));

vi.mock("../state", () => ({
  useAppState: () => ({
    user: null,
    business: null,
    setUser: vi.fn(),
    setBusiness: vi.fn(),
  }),
}));

vi.mock("../api", () => ({
  api: {
    register: vi.fn(),
    login: vi.fn(),
    getLocalState: vi.fn(() => ({ business: {} })),
    completeOnboarding: vi.fn(),
  },
  ApiError: class ApiError extends Error {},
}));

describe("AuthScreen mode switch semantics (F-15 #11)", () => {
  it("exposes the active auth mode via aria-pressed", () => {
    render(<AuthScreen />);
    const login = screen.getByRole("button", { name: "Masuk (Sign In)" });
    const register = screen.getByRole("button", { name: "Buka Buku Baru" });
    expect(login).toHaveAttribute("aria-pressed", "true");
    expect(register).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(register);
    expect(login).toHaveAttribute("aria-pressed", "false");
    expect(register).toHaveAttribute("aria-pressed", "true");
  });
});

describe("OnboardingScreen step indicator (F-15 #11)", () => {
  it("marks the current step with aria-current=step and moves focus between steps", () => {
    render(<OnboardingScreen />);

    const pill = (label: string) => screen.getByText(label).parentElement!;
    expect(pill("Profil Usaha")).toHaveAttribute("aria-current", "step");
    expect(pill("Periode Buku")).not.toHaveAttribute("aria-current");

    fireEvent.change(screen.getByLabelText(/Nama Entitas/), {
      target: { value: "PT QA A11y" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Lanjutkan ke Periode Buku/ }));

    expect(pill("Periode Buku")).toHaveAttribute("aria-current", "step");
    expect(pill("Profil Usaha")).not.toHaveAttribute("aria-current");

    const heading = screen.getByRole("heading", { name: /Langkah 2:/ });
    expect(document.activeElement).toBe(heading);

    fireEvent.click(screen.getByRole("button", { name: /Kembali/ }));
    expect(document.activeElement).toBe(screen.getByRole("heading", { name: /Langkah 1:/ }));
  });
});
