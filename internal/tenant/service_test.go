package tenant_test

import (
	"testing"

	"github.com/CodeEnthusiast09/proctura-backend/internal/mailer"
	"github.com/CodeEnthusiast09/proctura-backend/internal/models"
	"github.com/CodeEnthusiast09/proctura-backend/internal/tenant"
	"github.com/CodeEnthusiast09/proctura-backend/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreate_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := tenant.NewService(db, &mailer.NoOpMailer{}, "http://localhost:3000")
	ten, admin, err := svc.Create(tenant.CreateTenantInput{
		Name:           "University of Lagos",
		Subdomain:      "unilag",
		AdminEmail:     "admin@unilag.edu",
		AdminFirstName: "Chidi",
		AdminLastName:  "Okeke",
	})

	require.NoError(t, err)
	assert.Equal(t, "unilag", ten.Subdomain)
	assert.Equal(t, models.RoleSchoolAdmin, admin.Role)
	assert.NotNil(t, admin.InviteToken)
	assert.False(t, admin.IsVerified)
}

func TestCreate_DuplicateSubdomain(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := tenant.NewService(db, &mailer.NoOpMailer{}, "http://localhost:3000")
	_, _, err := svc.Create(tenant.CreateTenantInput{
		Name:           "School A",
		Subdomain:      "schoola",
		AdminEmail:     "admin@schoola.edu",
		AdminFirstName: "Admin",
		AdminLastName:  "One",
	})
	require.NoError(t, err)

	_, _, err = svc.Create(tenant.CreateTenantInput{
		Name:           "School B",
		Subdomain:      "schoola", // duplicate
		AdminEmail:     "admin@schoolb.edu",
		AdminFirstName: "Admin",
		AdminLastName:  "Two",
	})
	assert.ErrorIs(t, err, tenant.ErrSubdomainTaken)
}

func TestCreate_DuplicateAdminEmail(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := tenant.NewService(db, &mailer.NoOpMailer{}, "http://localhost:3000")
	_, _, err := svc.Create(tenant.CreateTenantInput{
		Name:           "School A",
		Subdomain:      "schoola",
		AdminEmail:     "same@email.com",
		AdminFirstName: "Admin",
		AdminLastName:  "One",
	})
	require.NoError(t, err)

	_, _, err = svc.Create(tenant.CreateTenantInput{
		Name:           "School B",
		Subdomain:      "schoolb",
		AdminEmail:     "same@email.com", // duplicate email
		AdminFirstName: "Admin",
		AdminLastName:  "Two",
	})
	assert.ErrorIs(t, err, tenant.ErrAdminEmailTaken)
}

func TestList_Pagination(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := tenant.NewService(db, &mailer.NoOpMailer{}, "http://localhost:3000")
	for i := 0; i < 5; i++ {
		_, _, err := svc.Create(tenant.CreateTenantInput{
			Name:           "School",
			Subdomain:      "school" + string(rune('a'+i)),
			AdminEmail:     "admin" + string(rune('a'+i)) + "@test.com",
			AdminFirstName: "Admin",
			AdminLastName:  "User",
		})
		require.NoError(t, err)
	}

	tenants, total, err := svc.List(1, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, tenants, 3)
}

func TestUpdate_Success(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := tenant.NewService(db, &mailer.NoOpMailer{}, "http://localhost:3000")
	ten, _, err := svc.Create(tenant.CreateTenantInput{
		Name:           "Old Name",
		Subdomain:      "oldname",
		AdminEmail:     "admin@oldname.edu",
		AdminFirstName: "Admin",
		AdminLastName:  "User",
	})
	require.NoError(t, err)

	updated, err := svc.Update(ten.ID, "New Name", nil)
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
}

func TestDelete_NotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupTables(t, db)

	svc := tenant.NewService(db, &mailer.NoOpMailer{}, "http://localhost:3000")
	err := svc.Delete("00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, tenant.ErrTenantNotFound)
}
