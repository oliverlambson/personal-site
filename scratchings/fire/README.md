# fire

Simulation of organisation quality and output with time.

An organisation is a tree of employees that changes over time. Each employee has
a quality rating from 0-1 (0 bad, 1 good). Each employee has a target number of
total subordinates that has a growth rate, and a target number of direct
reports. Total hires over time is the organisation's growth rate. At each point
in time, hires are distributed down the tree, with preference going to the
employees with the biggest delta between target and actual number of
subordinates. If an employee hires someone, the quality of the person they hire
is a function of their quality plus randomness. Each employee has a probability
of leaving which is a function of their quality plus randomness. They have a
probability of being fired which is a function of their quality plus randomness.
If they leave or are fired, they can either be replaced with an external person,
replaced by a subordinate, or not replaced and their subordinates need to move
to someone else.
