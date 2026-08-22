import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { SegmentedAgingBar } from "./SegmentedAgingBar";
import { TrendPill } from "./TrendPill";
import { QuickRatioGauge } from "./QuickRatioGauge";

describe("Analytics Components", () => {
  describe("SegmentedAgingBar", () => {
    it("renders empty state when all buckets are 0", () => {
      const { container } = render(
        <SegmentedAgingBar buckets={{ b0_30: 0, b31_60: 0, b61_90: 0, over90: 0 }} />
      );
      const bar = container.querySelector(".aging-stacked-bar--empty");
      expect(bar).toBeInTheDocument();
    });

    it("renders segments when buckets have values", () => {
      const { container } = render(
        <SegmentedAgingBar
          buckets={{ b0_30: 1000, b31_60: 2000, b61_90: 0, over90: 1000 }}
        />
      );
      const bar = container.querySelector(".aging-stacked-bar");
      expect(bar).toBeInTheDocument();
      // Should have 3 segment children (since 61_90 is 0)
      expect(bar?.children.length).toBe(3);
    });
  });

  describe("TrendPill", () => {
    it("returns null when deltaPct is null", () => {
      const { container } = render(<TrendPill deltaPct={null} />);
      expect(container.firstChild).toBeNull();
    });

    it("renders positive percentage with plus symbol", () => {
      render(<TrendPill deltaPct={12.4} label="MoM" />);
      expect(screen.getByText("▲ +12.4%")).toBeInTheDocument();
      expect(screen.getByText("MoM")).toBeInTheDocument();
    });

    it("renders negative percentage with minus symbol", () => {
      render(<TrendPill deltaPct={-3.14} />);
      expect(screen.getByText("▼ 3.1%")).toBeInTheDocument();
    });
  });

  describe("QuickRatioGauge", () => {
    it("renders gauge and labels healthy condition", () => {
      render(<QuickRatioGauge value={2.5} />);
      expect(screen.getByText(/Sehat/i)).toBeInTheDocument();
    });

    it("renders gauge and labels danger condition", () => {
      render(<QuickRatioGauge value={0.8} />);
      expect(screen.getByText(/Kritis/i)).toBeInTheDocument();
    });

    it("renders gauge and labels warning condition", () => {
      render(<QuickRatioGauge value={1.1} />);
      expect(screen.getByText(/Waspada/i)).toBeInTheDocument();
    });
  });
});
