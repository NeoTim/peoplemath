package main

import (
	"context"
	"fmt"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/datastore"
)

// StorageService using Google Cloud Datastore
type googleCDSStore struct {
	client *datastore.Client
}

func makeGoogleCDSStore(ctx context.Context, projectID string) (StorageService, error) {
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("Could not create datastore client: %s", err)
	}
	return &googleCDSStore{client: client}, nil
}

func getTeamKey(teamID string) *datastore.Key {
	return datastore.NameKey("Team", teamID, nil)
}

func getPeriodKey(teamKey *datastore.Key, periodID string) *datastore.Key {
	return datastore.NameKey("Period", periodID, teamKey)
}

func (s *googleCDSStore) GetAllTeams(ctx context.Context) ([]Team, error) {
	query := datastore.NewQuery("Team").Order("DisplayName")
	iter := s.client.Run(ctx, query)
	result := []Team{}
	for {
		var t Team
		_, err := iter.Next(&t)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return result, err
		}
		result = append(result, t)
	}
	return result, nil
}

func (s *googleCDSStore) GetTeam(ctx context.Context, teamID string) (Team, bool, error) {
	key := getTeamKey(teamID)
	var team Team
	err := s.client.Get(ctx, key, &team)
	if err == datastore.ErrNoSuchEntity {
		return team, false, nil
	}
	if err != nil {
		return team, true, err
	}
	return team, true, nil
}

func (s *googleCDSStore) CreateTeam(ctx context.Context, team Team) error {
	key := getTeamKey(team.ID)
	_, err := s.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var empty Team
		if err := tx.Get(key, &empty); err != datastore.ErrNoSuchEntity {
			return fmt.Errorf("Expected no existing team '%s', found: %s", team.ID, err)
		}
		_, err := tx.Put(key, &team)
		return err
	})
	return err
}

func (s *googleCDSStore) UpdateTeam(ctx context.Context, team Team) error {
	key := getTeamKey(team.ID)
	_, err := s.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var ignored Team
		if err := tx.Get(key, &ignored); err != nil {
			return fmt.Errorf("Could not retrieve team '%s': %s", team.ID, err)
		}
		_, err := tx.Put(key, &team)
		return err
	})
	return err
}

func (s *googleCDSStore) GetAllPeriods(ctx context.Context, teamID string) ([]Period, bool, error) {
	if _, ok, err := s.GetTeam(ctx, teamID); !ok || err != nil {
		return nil, false, err
	}
	teamKey := getTeamKey(teamID)
	query := datastore.NewQuery("Period").Ancestor(teamKey)
	iter := s.client.Run(ctx, query)
	result := []Period{}
	for {
		var p Period
		_, err := iter.Next(&p)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return result, true, err
		}
		scrubLoadedPeriod(&p)
		result = append(result, p)
	}
	return result, true, nil
}

func (s *googleCDSStore) GetPeriod(ctx context.Context, teamID, periodID string) (Period, bool, error) {
	teamKey := getTeamKey(teamID)
	periodKey := getPeriodKey(teamKey, periodID)
	var period Period
	err := s.client.Get(ctx, periodKey, &period)
	if err == datastore.ErrNoSuchEntity {
		return period, false, nil
	}
	scrubLoadedPeriod(&period)
	return period, true, err
}

func (s *googleCDSStore) CreatePeriod(ctx context.Context, teamID string, period Period) error {
	teamKey := getTeamKey(teamID)
	periodKey := getPeriodKey(teamKey, period.ID)
	_, err := s.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var empty Period
		if err := tx.Get(periodKey, &empty); err != datastore.ErrNoSuchEntity {
			return fmt.Errorf("Expected no period '%s' for team '%s': %s", period.ID, teamID, err)
		}
		_, err := tx.Put(periodKey, &period)
		return err
	})
	return err
}

func (s *googleCDSStore) UpdatePeriod(ctx context.Context, teamID string, period Period) error {
	teamKey := getTeamKey(teamID)
	periodKey := getPeriodKey(teamKey, period.ID)
	_, err := s.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var ignored Period
		if err := tx.Get(periodKey, &ignored); err != nil {
			return fmt.Errorf("Could not retrieve period '%s' for team '%s': %s", period.ID, teamID, err)
		}
		_, err := tx.Put(periodKey, &period)
		return err
	})
	return err
}

func (s *googleCDSStore) Close() error {
	return s.client.Close()
}

// Cloud Datastore does not save zero-length slices, so when retrieving entities with slice members,
// they may be nil. This function is to avoid clients having to deal with this.
func scrubLoadedPeriod(period *Period) {
	if period.People == nil {
		period.People = []Person{}
	}
	if period.Buckets == nil {
		period.Buckets = []Bucket{}
	}
	for i := range period.Buckets {
		if period.Buckets[i].Objectives == nil {
			period.Buckets[i].Objectives = []Objective{}
		}
		for j := range period.Buckets[i].Objectives {
			if period.Buckets[i].Objectives[j].Assignments == nil {
				period.Buckets[i].Objectives[j].Assignments = []Assignment{}
			}
		}
	}
}