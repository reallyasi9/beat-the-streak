using ProgressMeter

in_file = ARGS[1]
data = readcsv(in_file, Any, header=true, comment_char='%')

teams = data[1][:,1]
nteams = length(teams)

probs = data[1][:,2:end]
nweeks = size(probs, 2)
conv_probs = similar(probs, Float32)
for x in 1:size(probs,1), y in 1:size(probs,2)
  conv_probs[x,y] = probs[x,y] == "#N/A" ? NaN : probs[x,y]
end

#=
Goal: keep randomly shuffling the selected order of streaks,
rejecting invalid streaks (can't choose a team that has a bye),
then adding the remaining doubled-down team randomly, again
rejecting invalid streaks.  Multiply up the probability, check it
against the current best, then report if we found the best so far.
=#

@everywhere function fill_probabilities!(plist::Array{Float32}, teams::Array{Int}, probs::Array{Float32})
  for i in 1:length(teams)
    @inbounds p = probs[teams[i], i]
    if isnan(p)
      break
    end
    push!(plist, p)
  end
end

@everywhere function find_dd(probs::Array{Float32}, teams::Array{Int64}, combined_teams::Array{Int64})
  dd_week = findfirst(teams, combined_teams[1])
  dd_prob = probs[combined_teams[2], dd_week]
  return (dd_prob, dd_week)
end

# List inversion: p[i]=j => p[j]=i
@everywhere function invert!(p::Array{Int})
  setindex!(p, 1:length(p), p)
end

# Output reducer
@everywhere function max_prob(lhs::Array,
                              rhs::Array)
  if length(lhs) == 0
    return length(rhs) == 0 ? lhs : rhs
  elseif length(rhs) == 0
    return lhs
  else
    return lhs[end] < rhs[end] ? rhs : lhs
  end
end

# If there were no bye weeks, there would be a total of nCr(nteams, 2) * nPr(nteams-1, nweeks) streaks.
# Note that nteams-1 == nweeks.
combs = combinations(1:nteams, 2) # does nCr(nteams, 2) when iterated over
vcombs = collect(combs)
@sync @parallel for i in 1:length(vcombs)

  # Begin the output
  out_stream = open("week1_streak_output_$i.csv", "w")
  writedlm(out_stream, ["time" "combination" "permutation" transpose(teams) "probability"], ',')
  close(out_stream)

  combined_teams = vcombs[i]
  remaining_teams = collect(1:nteams)
  splice!(remaining_teams, combined_teams[2]) # remove the second of the teams that share a week

  println("Combination ", i, ": ", remaining_teams, " + ", combined_teams)

  nperms = factorial(nweeks)
  perms = permutations(remaining_teams)

  j = 0
  best_prob = 0.
  @showprogress 10 "Permuting..." for streak_team_order in perms
    j += 1

    plist = Float32[]
    fill_probabilities!(plist, streak_team_order, conv_probs)
    if length(plist) == nweeks

      (dd_prob, dd_week) = find_dd(conv_probs, streak_team_order, combined_teams)
      push!(plist, dd_prob)

      plist = cumprod(plist)

      if plist[end] > best_prob
        best_prob = plist[end]
        best_streak = teams[streak_team_order]
        best_dd_team = teams[combined_teams[2]]
        println("($best_prob) New best @ permutation $j: $best_streak + $best_dd_team in week $dd_week")

        # push in the missing team so the inversion works
        push!(streak_team_order, combined_teams[2])
        invert!(streak_team_order)
        # now replace the week number of the dd team with its companion
        streak_team_order[combined_teams[2]] = streak_team_order[combined_teams[1]]

        out_stream = open("week1_streak_output_$i.csv", "a")
        writedlm(out_stream, [time() i j transpose(streak_team_order) best_prob], ',')
        close(out_stream)
      end

    end

  end

end
