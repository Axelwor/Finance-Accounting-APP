import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { useEffect } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { ToastProvider, useToast } from "./Toast";

function Harness() {
  const toast = useToast();
  useEffect(() => {
    toast.info("Disimpan");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return null;
}

describe("Toast pause-on-focus (F-15 #10)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("auto-dismisses after the duration", () => {
    render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    );
    expect(screen.getByText("Disimpan")).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(4100);
    });
    expect(screen.queryByText("Disimpan")).not.toBeInTheDocument();
  });

  it("holds the timer while any part of a toast has focus", () => {
    render(
      <ToastProvider>
        <Harness />
      </ToastProvider>,
    );
    const closeBtn = screen.getByRole("button", { name: /dismiss notification/i });

    // Focus arrives before the timer would fire.
    fireEvent.focus(closeBtn);
    act(() => {
      vi.advanceTimersByTime(10000);
    });
    expect(screen.getByText("Disimpan")).toBeInTheDocument();

    // Blur restarts the dismiss timer.
    fireEvent.blur(closeBtn);
    act(() => {
      vi.advanceTimersByTime(4100);
    });
    expect(screen.queryByText("Disimpan")).not.toBeInTheDocument();
  });
});
