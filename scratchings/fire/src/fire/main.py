"""
Top employee: 80% chance top, 20% chance Ave
Ace employees: 20% chance top, 60% change Ave, 20% bad

Overall firm hiring process scalar
Churn within team
team growth
Team depth (how many people that person hires)
Role sensitivity to quality, what is output if good vs bad?

What is average quality or output after x years or at y people
"""

from __future__ import annotations

from typing import Literal, NamedTuple


class Employee(NamedTuple):
    performance: Literal["top", "ave", "bad"]
    p_churn: float
    p_top: float
    p_ave: float
    p_bad: float


class Team(NamedTuple):
    n_employees: int
    n_teams: int
